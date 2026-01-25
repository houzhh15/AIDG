#!/usr/bin/env python3
"""
生成并维护说话人向量中心:

使用 diarization (speakers.json) 输出 + 原始音频，提取每个说话人所有片段的嵌入向量并聚合为中心向量；
可加载已有 embeddings JSON 做跨文件说话人标签匹配与延续。

基本思路:
1. 读取 speakers.json: {"segments":[{"start":..,"end":..,"speaker":"SPK00"}, ...]}
2. 对相同 speaker 的所有片段提取嵌入 (pyannote/embedding)；每段向量加权平均(权重=时长)得到说话人中心；L2 归一化。
3. 若提供 --existing-embeddings，计算与已有中心的余弦相似度(向量已规范化则直接点积)，> 阈值(默认0.75)则复用旧标签；否则新建标签。
4. 对复用的标签做增量更新(按时长加权重新计算中心)以提升鲁棒性。
5. 输出：
   {
     "speakers": [ {"speaker":"SPK00","embedding":[...],"duration":123.4}, ...],
     "mapping": {"FILE_SPK00":"SPK00", "FILE_SPK01":"SPK02", ...},
     "model": "pyannote/embedding",
     "threshold": 0.75
   }

用法:
  python3 generate_speaker_embeddings.py \
      --audio example/test5.m4a \
      --speakers-json output/test5_speakers.json \
      --output output/test5_embeddings.json \
      --existing-embeddings output/meeting_embeddings.json \
      --hf_token <TOKEN> --device auto

离线:
  已缓存后可加 --offline 或设置 HF_HUB_OFFLINE=1；本地目录模型可用 --embedding-model /path/to/local_dir

依赖: pyannote.audio torch soundfile numpy huggingface_hub
"""
from __future__ import annotations

import argparse
import json
import os
import sys
import math
import platform
import shutil
import tempfile
import subprocess
from pathlib import Path
from typing import Dict, List, Tuple

import numpy as np
import soundfile as sf

# =============================================================================
# CRITICAL: Apply torch.load patch BEFORE importing pyannote or lightning
# PyTorch 2.6+ changed default weights_only=True, breaking pyannote models
# =============================================================================
try:
    import torch
    
    _original_torch_load = torch.load
    
    def _patched_torch_load(f, *args, **kwargs):
        """Patched torch.load that forces weights_only=False for compatibility."""
        if 'weights_only' not in kwargs or kwargs.get('weights_only') is None:
            kwargs['weights_only'] = False
        return _original_torch_load(f, *args, **kwargs)
    
    # Apply the patch globally
    torch.load = _patched_torch_load
    
    # Also patch lightning_fabric which pyannote uses
    try:
        import lightning_fabric.utilities.cloud_io as _cloud_io
        _original_cloud_io_load = _cloud_io._load
        
        def _patched_cloud_io_load(path_or_url, map_location=None, weights_only=None):
            if weights_only is None:
                weights_only = False
            return _original_cloud_io_load(path_or_url, map_location=map_location, weights_only=weights_only)
        
        _cloud_io._load = _patched_cloud_io_load
    except ImportError:
        pass
        
except Exception as e:
    print(json.dumps({"error": f"torch not available: {e}"}, ensure_ascii=False))
    sys.exit(1)


# --- 音频转换：与 pyannote_diarize 保持一致 ---
def ensure_wav_mono_16k(src_path: str) -> str:
    """将输入音频转换为临时 16k 单声道 WAV 文件，优先使用 ffmpeg，失败则回退 librosa+soundfile。
    返回转换后文件路径。"""
    tmpdir = tempfile.mkdtemp(prefix="spk_emb_wav_")
    dst_path = os.path.join(tmpdir, "audio_16k.wav")

    ffmpeg = shutil.which("ffmpeg")
    if ffmpeg:
        try:
            cmd = [
                ffmpeg,
                "-nostdin", "-hide_banner", "-loglevel", "error",
                "-y",
                "-i", src_path,
                "-ac", "1",
                "-ar", "16000",
                dst_path,
            ]
            subprocess.check_call(cmd)
            return dst_path
        except Exception as e:
            print(f"ffmpeg 转换失败: {e}", file=sys.stderr)

    # fallback librosa
    try:
        import librosa
        y, _sr = librosa.load(src_path, sr=16000, mono=True)
        sf.write(dst_path, y, 16000)
        return dst_path
    except Exception as e:
        raise RuntimeError(f"fallback 转换失败: {e}")


def parse_args():
    p = argparse.ArgumentParser()
    p.add_argument("--audio", required=True, help="原始音频路径")
    p.add_argument("--speakers-json", required=True, help="diarization 输出 JSON 文件")
    p.add_argument("--output", required=True, help="输出 embeddings JSON")
    p.add_argument("--existing-embeddings", help="已有 embeddings JSON, 用于延续标签")
    p.add_argument("--embedding-model", default="pyannote/embedding", help="嵌入模型 repo id 或本地目录")
    p.add_argument("--hf_token", default=os.getenv("HUGGINGFACE_TOKEN"))
    p.add_argument("--device", default="auto", choices=["auto", "cpu", "mps", "cuda"], help="计算设备")
    p.add_argument("--offline", action="store_true", help="离线模式，仅使用缓存")
    p.add_argument("--cache_dir", default=None, help="HF 缓存目录")
    p.add_argument("--threshold", type=float, default=0.75, help="余弦相似度阈值, >= 此值复用旧标签")
    p.add_argument("--min-segment-dur", type=float, default=0.3, help="忽略短于该秒数的片段 (并用于最小填充长度)；默认0.3s 以避免卷积核报错")
    p.add_argument("--similarity-matrix-output", help="将新文件说话人与已有全局说话人的相似度矩阵输出到该 JSON 文件 (可选)")
    p.add_argument("--auto-lower-threshold", action="store_true", help="若没有任何匹配则自动逐步降低阈值直至出现匹配或达到下限")
    p.add_argument("--auto-lower-min", type=float, default=0.60, help="自动降阈值时的下限 (默认0.60)")
    p.add_argument("--auto-lower-step", type=float, default=0.02, help="自动降阈值步长 (默认0.02)")
    p.add_argument("--file-speaker-report", help="输出每个文件说话人的最佳匹配及相似度统计 JSON")
    p.add_argument("--min-rms", type=float, default=0.002, help="最小 RMS 能量阈值, 低于此能量的片段将被忽略 (默认0.002)")
    p.add_argument("--rms-min-frames", type=int, default=1, help="计算 RMS 时至少包含帧数 (窗口=全部, 此参数用于一致性)")
    p.add_argument("--target-local-speakers", type=int, default=0, help="期望该文件本地说话人数量 (>0 启用). 若初始标签数超过此值, 将按相似度迭代合并最相似的两个.")
    # 片段离群点清理
    p.add_argument("--intra-clean", action="store_true", help="启用单说话人内部片段相似度离群点清理")
    p.add_argument("--intra-clean-percentile", type=float, default=0.25, help="低于该相似度分位的片段将被剔除 (默认0.25 即 p25)")
    p.add_argument("--intra-clean-min-segments", type=int, default=3, help="每个说话人至少保留的片段数，避免过度清理")
    p.add_argument("--intra-clean-max-iter", type=int, default=2, help="最大迭代清理轮次")
    return p.parse_args()


def _find_local_hf_cache_root() -> str:
    """Best-effort locate a HuggingFace cache root directory.

    Priority:
      1) $HF_HOME if points to an existing dir
      2) $TRANSFORMERS_CACHE if points to an existing dir
      3) Search upwards from this file for a 'models/huggingface' directory
    """
    for key in ("HF_HOME", "TRANSFORMERS_CACHE"):
        v = os.getenv(key)
        if v and os.path.isdir(v):
            return v

    here = Path(__file__).resolve()
    for parent in list(here.parents)[:12]:
        cand = parent / "models" / "huggingface"
        if cand.is_dir():
            return str(cand)
    return ""


def _repo_id_to_cache_dirname(repo_id: str) -> str:
    # 'pyannote/embedding' -> 'models--pyannote--embedding'
    return "models--" + repo_id.replace("/", "--")


def _resolve_hf_snapshot_dir(repo_id: str, cache_root: str) -> str:
    """Resolve a repo_id to a local snapshot directory inside HF hub cache."""
    if not repo_id or not cache_root:
        return ""
    hub_root = Path(cache_root) / "hub"
    model_root = hub_root / _repo_id_to_cache_dirname(repo_id)
    snapshots = model_root / "snapshots"
    if not snapshots.is_dir():
        return ""

    snap_dirs = [p for p in snapshots.iterdir() if p.is_dir()]
    if not snap_dirs:
        return ""
    snap_dirs.sort(key=lambda p: p.stat().st_mtime, reverse=True)
    return str(snap_dirs[0])


def resolve_device(pref: str) -> str:
    if pref in {"cpu", "mps", "cuda"}:
        return pref
    try:
        if platform.system() == "Darwin" and torch.backends.mps.is_available():
            return "mps"
        if torch.cuda.is_available():
            return "cuda"
    except Exception:
        pass
    return "cpu"


def load_speakers_json(path: str):
    try:
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except Exception as e:
        raise RuntimeError(f"无法读取 speakers-json: {e}")
    if "error" in data and data["error"]:
        raise RuntimeError(f"speakers-json 含错误: {data['error']}")
    segments = data.get("segments", [])
    return segments


def group_segments_by_speaker(segments):
    spk_map = {}
    for seg in segments:
        # 支持大写和小写的键名
        spk = seg.get("speaker") or seg.get("Speaker")
        if spk is None:
            continue
        # 支持大写和小写的键名
        start = float(seg.get("start", seg.get("Start", 0.0)))
        end = float(seg.get("end", seg.get("End", 0.0)))
        if end <= start:
            continue
        spk_map.setdefault(spk, []).append((start, end))
    return spk_map


def load_existing_embeddings(path: str):
    if not path or not os.path.isfile(path):
        return {}
    try:
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except Exception:
        return {}
    speakers = {}
    for item in data.get("speakers", []):
        emb = np.array(item.get("embedding", []), dtype=np.float32)
        if emb.size == 0:
            continue
        # 归一化确保为单位向量
        norm = np.linalg.norm(emb)
        if norm == 0:
            continue
        emb = emb / norm
        speakers[item.get("speaker")] = {
            "embedding": emb,
            "duration": float(item.get("duration", 0.0))
        }
    return speakers


def maybe_offline_env(args):
    if args.offline:
        os.environ.setdefault("HF_HUB_OFFLINE", "1")


def _apply_offline_patches(hub_cache_dir: str):
    """
    Apply monkey-patches required for offline mode with pyannote.audio.
    
    This handles two issues:
    1. pyannote uses its own cache_dir instead of the configured one
    2. PyTorch 2.6+ defaults to weights_only=True which breaks older checkpoints
    
    Returns a cleanup function to restore original behavior.
    """
    import huggingface_hub
    import huggingface_hub.file_download
    
    # Store original functions
    original_hf_hub_download = huggingface_hub.file_download.hf_hub_download
    original_torch_load = torch.load
    
    def patched_hf_hub_download(*args, **kwargs):
        kwargs['local_files_only'] = True
        if hub_cache_dir:
            kwargs['cache_dir'] = hub_cache_dir
        kwargs.pop('use_auth_token', None)
        kwargs.pop('token', None)
        return original_hf_hub_download(*args, **kwargs)
    
    def patched_torch_load(*args, **kwargs):
        kwargs['weights_only'] = False
        return original_torch_load(*args, **kwargs)
    
    # Apply patches
    huggingface_hub.hf_hub_download = patched_hf_hub_download
    huggingface_hub.file_download.hf_hub_download = patched_hf_hub_download
    torch.load = patched_torch_load
    
    def cleanup():
        huggingface_hub.hf_hub_download = original_hf_hub_download
        huggingface_hub.file_download.hf_hub_download = original_hf_hub_download
        torch.load = original_torch_load
    
    return cleanup, patched_hf_hub_download, patched_torch_load


def load_embedding_model(args):
    cache_dir_explicit = args.cache_dir is not None

    # Default cache dir if not provided
    if args.cache_dir is None:
        found = _find_local_hf_cache_root()
        if found:
            args.cache_dir = found

    # Determine hub cache directory
    hub_cache_dir = None
    if args.cache_dir and os.path.isdir(args.cache_dir):
        os.environ.setdefault("HF_HOME", args.cache_dir)
        os.environ.setdefault("TRANSFORMERS_CACHE", args.cache_dir)
        os.environ.setdefault("TORCH_HOME", args.cache_dir)
        hub_cache = str(Path(args.cache_dir) / "hub")
        if os.path.isdir(hub_cache):
            hub_cache_dir = hub_cache
        else:
            hub_cache_dir = args.cache_dir
        os.environ.setdefault("HF_HUB_CACHE", hub_cache_dir)
        os.environ.setdefault("HUGGINGFACE_HUB_CACHE", hub_cache_dir)

    # 尝试本地目录或在线
    if os.path.isdir(args.embedding_model):
        cfg = Path(args.embedding_model)
        # Apply patches for torch.load compatibility
        cleanup, _, patched_torch_load = _apply_offline_patches(hub_cache_dir)
        try:
            from pyannote.audio import Model
            # Patch lightning_fabric's torch.load reference
            try:
                import lightning_fabric.utilities.cloud_io
                lightning_fabric.utilities.cloud_io.torch.load = patched_torch_load
            except:
                pass
            model = Model.from_pretrained(str(cfg))
            return model
        except Exception as e:
            raise RuntimeError(f"本地嵌入模型加载失败: {e}")
        finally:
            cleanup()
    
    # 如果指定了 --offline，强制离线模式
    if args.offline:
        os.environ["HF_HUB_OFFLINE"] = "1"
        os.environ.setdefault("TRANSFORMERS_OFFLINE", "1")
        
        # Apply all patches for offline mode
        cleanup, patched_hf_hub_download, patched_torch_load = _apply_offline_patches(hub_cache_dir)
        
        try:
            # Clear any previously imported pyannote modules
            import sys as _sys
            pyannote_modules = [k for k in _sys.modules.keys() if k.startswith('pyannote')]
            for mod in pyannote_modules:
                del _sys.modules[mod]
            lightning_modules = [k for k in _sys.modules.keys() if k.startswith('lightning')]
            for mod in lightning_modules:
                del _sys.modules[mod]
            
            # Now import pyannote with patches in place
            from pyannote.audio import Model
            
            # Patch module-level references
            try:
                import pyannote.audio.core.model
                pyannote.audio.core.model.hf_hub_download = patched_hf_hub_download
            except:
                pass
            try:
                import lightning_fabric.utilities.cloud_io
                lightning_fabric.utilities.cloud_io.torch.load = patched_torch_load
            except:
                pass
            
            model = Model.from_pretrained(
                args.embedding_model,
                token=None,
                cache_dir=hub_cache_dir,
                local_files_only=True,
            )
            return model
        except Exception as e:
            raise RuntimeError(f"离线模式下找不到已缓存模型 '{args.embedding_model}': {e}\n"
                             f"请确保模型已经下载到缓存目录，或使用 --embedding-model 指定本地模型路径")
        finally:
            cleanup()
    
    # 在线模式：正常加载 (HF 缓存会复用; 若网络问题会抛异常)
    try:
        from pyannote.audio import Model
        model = Model.from_pretrained(
            args.embedding_model,
            token=args.hf_token,
            cache_dir=args.cache_dir if cache_dir_explicit else None,
        )
    except Exception as e:
        raise RuntimeError(f"嵌入模型加载失败: {e}")
    return model


def extract_embeddings(waveform: np.ndarray, sr: int, segments: List[tuple], inference: Inference, min_seg_dur: float, min_rms: float) -> Tuple[np.ndarray, float, List[Dict]]:
    """对一个 speaker 的所有片段提取嵌入并聚合 (时长加权平均)，并保留每段嵌入用于统计。

    返回:
      centroid: 归一化中心 (或 None)
      total_duration: 累计有效时长
      seg_embeddings: [{"embedding": np.ndarray(normalized), "duration": dur, "start": s_sec, "end": e_sec, "rms": rms}, ...]
    """
    total_dur = 0.0
    vec_acc = None
    seg_embeddings: List[Dict] = []
    
    for (start, end) in segments:
        dur = end - start
        if dur < min_seg_dur:
            continue
        s = int(start * sr)
        e = int(end * sr)
        if e <= s or s >= len(waveform):
            continue
        e = min(e, len(waveform))
        chunk = waveform[s:e]
        min_required = int(sr * min_seg_dur)
        if len(chunk) < min_required:
            if len(chunk) == 0:
                continue
            pad_width = min_required - len(chunk)
            chunk = np.pad(chunk, (0, pad_width), mode="constant")
        rms = math.sqrt(float(np.mean(chunk**2))) if len(chunk) > 0 else 0.0
        if rms < min_rms:
            continue
        
        # 嵌入提取错误处理
        try:
            tens = torch.tensor(chunk, dtype=torch.float32)
            if tens.dim() == 1:
                tens = tens.unsqueeze(0)
            with torch.no_grad():
                emb = inference({"waveform": tens, "sample_rate": sr})
        except Exception as e:
            print(f"Warning: Failed to extract embedding for segment {start:.2f}-{end:.2f}s: {e}", file=sys.stderr)
            continue
            
        if isinstance(emb, (list, tuple)) and len(emb) > 0:
            emb = emb[0]
        if isinstance(emb, torch.Tensor):
            emb = emb.squeeze().detach().cpu().numpy().astype(np.float32)
        elif isinstance(emb, np.ndarray):
            emb = np.squeeze(emb).astype(np.float32)
        else:
            print(f"Warning: Unknown embedding type {type(emb)} for segment {start:.2f}-{end:.2f}s", file=sys.stderr)
            continue
            
        if not np.all(np.isfinite(emb)):
            print(f"Warning: Non-finite embedding for segment {start:.2f}-{end:.2f}s", file=sys.stderr)
            continue
            
        if vec_acc is None:
            vec_acc = emb * dur
        else:
            vec_acc += emb * dur
        total_dur += dur
        seg_norm = emb.copy()
        norm_seg = np.linalg.norm(seg_norm)
        if norm_seg > 0:
            seg_norm = seg_norm / norm_seg
        seg_embeddings.append({
            "embedding": seg_norm,
            "raw_embedding": emb,
            "duration": dur,
            "start": float(start),
            "end": float(end),
            "rms": rms
        })
    if total_dur == 0 or vec_acc is None:
        return None, 0.0, []
    centroid = vec_acc / total_dur
    norm = np.linalg.norm(centroid)
    if norm > 0:
        centroid = centroid / norm
    return centroid, total_dur, seg_embeddings


def cosine(a: np.ndarray, b: np.ndarray) -> float:
    return float(np.dot(a, b))


def match_existing(new_centroid: np.ndarray, existing: Dict[str, dict]):
    best_label = None
    best_sim = -2.0
    for label, info in existing.items():
        sim = cosine(new_centroid, info["embedding"])
        if sim > best_sim:
            best_sim = sim
            best_label = label
    return best_label, best_sim


def build_similarity_matrix(new_profiles: Dict[str, dict], existing: Dict[str, dict]) -> Dict[str, Dict[str, float]]:
    matrix = {}
    if not existing:
        return matrix
    for file_spk, prof in new_profiles.items():
        row = {}
        emb = prof["embedding"]
        if not np.all(np.isfinite(emb)):
            # 标记整行 NaN 以保持可见性
            row = {g_label: float('nan') for g_label in existing.keys()}
            matrix[file_spk] = row
            continue
        for g_label, g_info in existing.items():
            g_emb = g_info["embedding"]
            if not np.all(np.isfinite(g_emb)):
                row[g_label] = float('nan')
            else:
                row[g_label] = cosine(emb, g_emb)
        matrix[file_spk] = row
    return matrix


def main():
    args = parse_args()
    maybe_offline_env(args)

    if not os.path.isfile(args.audio):
        print(json.dumps({"error": f"音频不存在: {args.audio}"}, ensure_ascii=False))
        sys.exit(1)
    segments = load_speakers_json(args.speakers_json)
    if not segments:
        # When no segments available, create empty embeddings file instead of failing
        # This allows the pipeline to continue gracefully
        empty_result = {
            "error": "无可用 segments",
            "speakers": {},
            "segment_embeddings": [],
            "file_speakers_summary": []
        }
        try:
            with open(args.output, "w", encoding="utf-8") as f:
                json.dump(empty_result, f, ensure_ascii=False, indent=2)
            print(json.dumps(empty_result, ensure_ascii=False))
        except Exception as e:
            print(json.dumps({"error": f"写入空结果失败: {e}"}, ensure_ascii=False))
        return

    # 标准化音频：转为 16k 单声道 wav
    try:
        wav_path = ensure_wav_mono_16k(args.audio)
    except Exception as e:
        print(json.dumps({"error": f"音频转换失败: {e}"}, ensure_ascii=False))
        sys.exit(1)

    # 读转换后的 wav (float32)
    try:
        waveform, sr = sf.read(wav_path)
    except Exception as e:
        print(json.dumps({"error": f"读取转换后音频失败: {e}"}, ensure_ascii=False))
        sys.exit(1)
    if waveform.ndim > 1:
        waveform = waveform.mean(axis=1)
    if waveform.dtype != np.float32:
        waveform = waveform.astype(np.float32)

    # 加载已有 embeddings
    existing = load_existing_embeddings(args.existing_embeddings) if args.existing_embeddings else {}

    # 加载嵌入模型 & 推理器
    try:
        model = load_embedding_model(args)
    except Exception as e:
        print(json.dumps({"error": str(e)}, ensure_ascii=False))
        sys.exit(1)

    try:
        from pyannote.audio import Inference
    except Exception as e:
        print(json.dumps({"error": f"pyannote.audio not available: {e}"}, ensure_ascii=False))
        sys.exit(1)
    device_str = resolve_device(args.device)
    # 将字符串转换为 torch.device
    try:
        if device_str == "cuda":
            torch_device = torch.device("cuda") if torch.cuda.is_available() else torch.device("cpu")
        elif device_str == "mps":
            torch_device = torch.device("mps") if (platform.system()=="Darwin" and torch.backends.mps.is_available()) else torch.device("cpu")
        else:
            torch_device = torch.device("cpu")
    except Exception:
        torch_device = torch.device("cpu")

    try:
        model.to(torch_device)
    except Exception:
        pass
    inference = Inference(model, window="whole", device=torch_device)

    # 按 speaker 分组
    spk_segments = group_segments_by_speaker(segments)
    print(f"Debug: Found {len(spk_segments)} speakers in segments", file=sys.stderr)
    for spk, segs in spk_segments.items():
        print(f"Debug: Speaker {spk} has {len(segs)} segments", file=sys.stderr)

    new_profiles = {}  # file local speaker -> (centroid, dur)
    for spk, seg_list in spk_segments.items():
        print(f"Debug: Processing speaker {spk} with {len(seg_list)} segments", file=sys.stderr)
        centroid, dur, seg_embs = extract_embeddings(waveform, sr, seg_list, inference, args.min_segment_dur, args.min_rms)
        if centroid is None:
            print(f"Warning: No valid embeddings for speaker {spk}", file=sys.stderr)
            continue
        print(f"Debug: Speaker {spk} centroid shape: {centroid.shape}, duration: {dur:.2f}s, segments: {len(seg_embs)}", file=sys.stderr)
        new_profiles[spk] = {"embedding": centroid, "duration": dur, "segments": seg_embs}

    # ---------------- 单说话人内部片段清理 (离群点剔除) ----------------
    intra_clean_log = []  # [{speaker, iterations:[{iter, n_before, n_after, cutoff, removed, mean_before, mean_after}]}]
    if args.intra_clean and new_profiles:
        from statistics import mean as _mean
        for spk, prof in list(new_profiles.items()):
            segs = prof.get("segments", [])
            if len(segs) < args.intra_clean_min_segments + 1:  # 太少不清理
                continue
            iter_logs = []
            for it in range(1, args.intra_clean_max_iter + 1):
                # 重新计算中心 (使用 raw embedding * duration 加权)
                total_dur = 0.0
                acc = None
                for sg in segs:
                    raw = sg.get("raw_embedding")
                    d = sg.get("duration", 0.0)
                    if raw is None or not np.all(np.isfinite(raw)) or d <= 0:
                        continue
                    if acc is None:
                        acc = raw * d
                    else:
                        acc += raw * d
                    total_dur += d
                if acc is None or total_dur == 0:
                    break
                centroid = acc / total_dur
                nrm = np.linalg.norm(centroid)
                if nrm > 0:
                    centroid = centroid / nrm
                # 计算每段相似度
                sims = []
                for idx, sg in enumerate(segs):
                    emb_norm = sg.get("embedding")
                    if emb_norm is None or centroid is None:
                        continue
                    if not (np.all(np.isfinite(emb_norm)) and np.all(np.isfinite(centroid))):
                        continue
                    sim = float(np.dot(emb_norm, centroid))
                    sims.append((idx, sim))
                if len(sims) < args.intra_clean_min_segments + 1:
                    break
                sim_values = np.array([s for (_, s) in sims], dtype=np.float32)
                mean_before = float(sim_values.mean())
                cutoff = np.percentile(sim_values, args.intra_clean_percentile * 100.0)
                # 标记将被移除的 idx
                remove_set = {idx for (idx, sim) in sims if sim < cutoff}
                # 保护：不能移除后剩余 < min_segments
                if len(segs) - len(remove_set) < args.intra_clean_min_segments:
                    break
                if not remove_set:
                    break
                # 执行移除
                new_segs = [sg for i, sg in enumerate(segs) if i not in remove_set]
                # 重新计算 mean_after
                sims_after = []
                for sg in new_segs:
                    emb_norm = sg.get("embedding")
                    if emb_norm is None:
                        continue
                    sim_a = float(np.dot(emb_norm, centroid)) if centroid is not None else 0.0
                    sims_after.append(sim_a)
                mean_after = float(_mean(sims_after)) if sims_after else mean_before
                iter_logs.append({
                    "iter": it,
                    "n_before": len(segs),
                    "n_after": len(new_segs),
                    "cutoff": float(round(cutoff, 6)),
                    "removed": len(remove_set),
                    "mean_before": round(mean_before, 6),
                    "mean_after": round(mean_after, 6)
                })
                segs = new_segs
                # 若无进一步可移除则停止
                if len(segs) < args.intra_clean_min_segments + 1:
                    break
            # 更新 profile
            if iter_logs:
                # 最终中心再计算一次
                total_dur = 0.0
                acc = None
                for sg in segs:
                    raw = sg.get("raw_embedding")
                    d = sg.get("duration", 0.0)
                    if raw is None or not np.all(np.isfinite(raw)) or d <= 0:
                        continue
                    if acc is None:
                        acc = raw * d
                    else:
                        acc += raw * d
                    total_dur += d
                if acc is not None and total_dur > 0:
                    centroid_final = acc / total_dur
                    nrm = np.linalg.norm(centroid_final)
                    if nrm > 0:
                        centroid_final = centroid_final / nrm
                    prof["embedding"] = centroid_final
                    prof["duration"] = total_dur
                    prof["segments"] = segs
                intra_clean_log.append({
                    "speaker": spk,
                    "iterations": iter_logs
                })
        # 移除清理后无效的说话人
        for spk in list(new_profiles.keys()):
            if not new_profiles[spk].get("segments"):
                del new_profiles[spk]

    # ---------------- 本地说话人碎片合并 (基于目标数量) ----------------
    local_merge_history = []  # [{"merge_pair":[A,B], "kept":A, "removed":B, "similarity":0.93, "new_duration":..., "target":N_after}, ...]
    local_original_mapping = {spk: spk for spk in new_profiles.keys()}  # 原始 -> 当前(可能被合并保留的标签)

    def recompute_pair_similarity(a_key, b_key):
        a = new_profiles[a_key]
        b = new_profiles[b_key]
        ea = a.get("embedding")
        eb = b.get("embedding")
        if ea is None or eb is None:
            return -2.0
        if not (np.all(np.isfinite(ea)) and np.all(np.isfinite(eb))):
            return -2.0
        return float(np.dot(ea, eb))

    if args.target_local_speakers and args.target_local_speakers > 0:
        # 仅在当前数量 > 目标时触发
        while len(new_profiles) > args.target_local_speakers:
            keys = list(new_profiles.keys())
            best_pair = None
            best_sim = -2.0
            for i in range(len(keys)):
                for j in range(i + 1, len(keys)):
                    k1, k2 = keys[i], keys[j]
                    sim = recompute_pair_similarity(k1, k2)
                    if sim > best_sim:
                        best_sim = sim
                        best_pair = (k1, k2)
            if best_pair is None:
                break  # 无法继续
            a_key, b_key = best_pair
            # 采用时长加权合并: centroid = (ea*dur_a + eb*dur_b)/(dur_a+dur_b) 后再归一化
            a_prof = new_profiles[a_key]
            b_prof = new_profiles[b_key]
            dur_a = a_prof.get("duration", 0.0)
            dur_b = b_prof.get("duration", 0.0)
            ea = a_prof["embedding"]
            eb = b_prof["embedding"]
            total_dur = dur_a + dur_b if (dur_a + dur_b) > 0 else 1.0
            merged = (ea * dur_a + eb * dur_b) / total_dur
            nrm = np.linalg.norm(merged)
            if nrm > 0:
                merged = merged / nrm
            # 合并 segments (保留全部段以利后续统计), 不再重新单段嵌入计算
            merged_segments = (a_prof.get("segments", []) or []) + (b_prof.get("segments", []) or [])
            # 选择保留标签: 取字典序较小者, 另一者移除
            keep, remove = (a_key, b_key) if a_key <= b_key else (b_key, a_key)
            keep_prof = new_profiles[keep]
            keep_prof["embedding"] = merged
            keep_prof["duration"] = total_dur
            keep_prof["segments"] = merged_segments
            # 删除被移除
            del new_profiles[remove]
            # 更新 original mapping: 原来映射到 remove 的都改到 keep
            for orig, cur in list(local_original_mapping.items()):
                if cur == remove:
                    local_original_mapping[orig] = keep
            # 记录合并历史
            local_merge_history.append({
                "merge_pair": [a_key, b_key],
                "kept": keep,
                "removed": remove,
                "similarity": round(best_sim, 6),
                "new_duration": total_dur,
                "remaining_after": len(new_profiles)
            })
            # 若没有足够的说话人进一步合并则退出
            if len(new_profiles) <= args.target_local_speakers:
                break

    if not new_profiles:
        print(json.dumps({"error": "未生成有效说话人向量"}, ensure_ascii=False))
        return

    # 生成全局标签映射
    # 已有的标签集合与最大编号
    existing_labels = list(existing.keys())
    max_index = -1
    for lbl in existing_labels:
        if lbl.startswith("SPK") and lbl[3:].isdigit():
            max_index = max(max_index, int(lbl[3:]))

    mapping = {}  # file speaker -> global speaker
    threshold = args.threshold

    def attempt_merge(thr: float) -> int:
        merged_count = 0
        for file_spk, prof in new_profiles.items():
            if file_spk in mapping:  # 已经匹配/创建
                continue
            centroid = prof["embedding"]
            # 跳过非有限值向量
            if centroid is None or not np.all(np.isfinite(centroid)):
                continue
            if existing:
                best_label, best_sim = match_existing(centroid, existing)
                if best_sim >= thr:
                    old = existing[best_label]
                    old_dur = old.get("duration", 0.0)
                    new_dur = prof["duration"]
                    if old_dur + new_dur > 0:
                        merged = (old["embedding"] * old_dur + centroid * new_dur) / (old_dur + new_dur)
                        norm = np.linalg.norm(merged)
                        if norm > 0:
                            merged = merged / norm
                        existing[best_label]["embedding"] = merged
                        existing[best_label]["duration"] = old_dur + new_dur
                    mapping[file_spk] = best_label
                    merged_count += 1
        return merged_count

    # 先用用户阈值
    attempt_merge(threshold)

    # 若无任何匹配且允许自动降阈值
    if args.auto_lower_threshold and existing and not any(v in existing for v in mapping.values()):
        cur = threshold - args.auto_lower_step
        while cur >= args.auto_lower_min and not any(v in existing for v in mapping.values()):
            attempt_merge(cur)
            if any(v in existing for v in mapping.values()):
                threshold = cur  # 记录实际匹配使用的阈值
                break
            cur -= args.auto_lower_step

    # 在新增标签之前，记录“旧”全局说话人集合，用于诊断相似度（避免自加入后自相似=1.0 干扰）
    existing_before_new = {k: v.copy() if isinstance(v, dict) else v for k, v in existing.items()}

    # 诊断矩阵：与旧全局说话人比较
    diag_sim_matrix = build_similarity_matrix(new_profiles, existing_before_new)

    # 基于诊断矩阵生成最佳匹配报告（仅针对旧说话人集合）
    best_report = []
    if diag_sim_matrix:
        for fspk, row in diag_sim_matrix.items():
            finite_items = {k: v for k, v in row.items() if isinstance(v, (int, float)) and np.isfinite(v)}
            if not finite_items:
                best_report.append({"file_speaker": fspk, "best_global": None, "best_sim": None})
            else:
                best_global = max(finite_items.items(), key=lambda x: x[1])
                best_report.append({"file_speaker": fspk, "best_global": best_global[0], "best_sim": best_global[1]})

    # 剩余未匹配的创建新标签（此后 existing 会包含新标签；不用于 best_report 计算）
    for file_spk, prof in new_profiles.items():
        if file_spk in mapping:
            continue
        centroid = prof["embedding"]
        if centroid is None or not np.all(np.isfinite(centroid)):
            continue
        max_index += 1
        new_label = f"SPK{max_index:02d}"
        existing[new_label] = {
            "embedding": centroid,
            "duration": prof["duration"]
        }
        mapping[file_spk] = new_label

    # 可选：完整矩阵（含新标签）若需要进一步分析，可保留；当前输出使用诊断矩阵避免自相似 1.0。
    sim_matrix = diag_sim_matrix

    # 构建输出
    # 记录无效(被跳过)的新文件说话人
    invalid_file_speakers = [spk for spk, prof in new_profiles.items() if prof["embedding"] is None or not np.all(np.isfinite(prof["embedding"]))]

    # 统计相似度分布（仅对 best_report 中有数值的条目）
    sims_for_stats = [r["best_sim"] for r in best_report if r.get("best_sim") is not None]
    sim_stats = {}
    suggested_threshold = None
    if sims_for_stats:
        arr = np.array(sims_for_stats, dtype=np.float32)
        sim_stats = {
            "count": int(arr.size),
            "mean": float(arr.mean()),
            "std": float(arr.std(ddof=0)),
            "min": float(arr.min()),
            "max": float(arr.max()),
            "p25": float(np.percentile(arr, 25)),
            "median": float(np.percentile(arr, 50)),
            "p75": float(np.percentile(arr, 75))
        }
        # 建议阈值：取 p25 与 (mean - 0.5*std) 的较大者，但不高于 p75
        candidate = max(sim_stats["p25"], sim_stats["mean"] - 0.5 * sim_stats["std"])
        candidate = min(candidate, sim_stats["p75"])
        suggested_threshold = float(round(candidate, 3))

    # ---------------- 类内 / 类间 统计 & 建议最低阈值 ----------------
    # 类内：各说话人每段 -> 其中心的相似度
    intra_records = []  # 所有段的相似度
    per_speaker_stats = []
    for fspk, prof in new_profiles.items():
        centroid = prof["embedding"]
        segs = prof.get("segments", [])
        sims = []
        for seg in segs:
            emb_seg = seg.get("embedding")
            if emb_seg is None or centroid is None:
                continue
            if not (np.all(np.isfinite(emb_seg)) and np.all(np.isfinite(centroid))):
                continue
            sim = float(np.dot(emb_seg, centroid))
            if np.isfinite(sim):
                sims.append(sim)
        if sims:
            arr = np.array(sims, dtype=np.float32)
            intra_records.extend(arr.tolist())
            per_speaker_stats.append({
                "file_speaker": fspk,
                "count": int(arr.size),
                "mean": float(arr.mean()),
                "std": float(arr.std(ddof=0)),
                "min": float(arr.min()),
                "max": float(arr.max()),
                "p25": float(np.percentile(arr, 25)),
                "median": float(np.percentile(arr, 50)),
                "p75": float(np.percentile(arr, 75))
            })
    intra_global = {}
    if intra_records:
        g = np.array(intra_records, dtype=np.float32)
        intra_global = {
            "count": int(g.size),
            "mean": float(g.mean()),
            "std": float(g.std(ddof=0)),
            "min": float(g.min()),
            "max": float(g.max()),
            "p25": float(np.percentile(g, 25)),
            "median": float(np.percentile(g, 50)),
            "p75": float(np.percentile(g, 75)),
            "p95": float(np.percentile(g, 95))
        }

    # 类间：不同说话人中心两两 cos（使用 旧全局 + 新文件中心 组合）
    inter_list = []
    combined_centroids = []
    # 旧全局
    for lbl, info in existing_before_new.items():
        emb = info.get("embedding") if isinstance(info, dict) else None
        if emb is not None and np.all(np.isfinite(emb)):
            combined_centroids.append((lbl, emb))
    # 新文件局部（使用 file speaker id 做标签前缀 FS_）
    for fspk, prof in new_profiles.items():
        emb = prof.get("embedding")
        if emb is not None and np.all(np.isfinite(emb)):
            combined_centroids.append((f"FILE::{fspk}", emb))
    for i in range(len(combined_centroids)):
        for j in range(i + 1, len(combined_centroids)):
            _, a = combined_centroids[i]
            _, b = combined_centroids[j]
            inter_list.append(float(np.dot(a, b)))
    inter_stats = {}
    if inter_list:
        inter_arr = np.array(inter_list, dtype=np.float32)
        inter_stats = {
            "count": int(inter_arr.size),
            "mean": float(inter_arr.mean()),
            "std": float(inter_arr.std(ddof=0)),
            "min": float(inter_arr.min()),
            "max": float(inter_arr.max()),
            "p25": float(np.percentile(inter_arr, 25)),
            "median": float(np.percentile(inter_arr, 50)),
            "p75": float(np.percentile(inter_arr, 75)),
            "p95": float(np.percentile(inter_arr, 95))
        }

    suggested_min_threshold = None
    if intra_global and inter_stats:
        cand = max(inter_stats.get("p95", 0.0), intra_global.get("mean", 0.0) - 2 * intra_global.get("std", 0.0))
        suggested_min_threshold = float(round(cand, 3))

    out = {
        "speakers": [
            {
                "speaker": lbl,
                "embedding": info["embedding"].tolist(),
                "duration": info.get("duration", 0.0)
            } for lbl, info in sorted(existing.items())
        ],
        "mapping": mapping,
        "model": args.embedding_model,
        "threshold": threshold,
        "source_audio": args.audio,
        "source_speakers_json": args.speakers_json,
        "similarity_matrix": sim_matrix if sim_matrix else {},
        "invalid_file_speakers": invalid_file_speakers,
        "file_speaker_best": best_report,
        "similarity_stats": sim_stats,
    "suggested_threshold": suggested_threshold,
    "intra_stats": {"global": intra_global, "per_speaker": per_speaker_stats} if intra_global else {},
    "inter_stats": inter_stats,
    "suggested_min_threshold": suggested_min_threshold,
    "local_merge_history": local_merge_history,
    "local_original_mapping": local_original_mapping,
    "target_local_speakers": args.target_local_speakers,
    "intra_clean_log": intra_clean_log
    }

    try:
        Path(args.output).parent.mkdir(parents=True, exist_ok=True)
        with open(args.output, "w", encoding="utf-8") as f:
            json.dump(out, f, ensure_ascii=False, indent=2)
    except Exception as e:
        print(json.dumps({"error": f"写入失败: {e}"}, ensure_ascii=False))
        sys.exit(1)

    # 同时打印摘要到 stdout (不含完整向量)
    summary = {
        "speakers": [s["speaker"] for s in out["speakers"]],
        "mapping": mapping,
        "output": args.output,
        "matched_existing": [v for v in mapping.values() if v in [sp["speaker"] for sp in out["speakers"]]],
    }
    # 可选输出相似度矩阵
    if args.similarity_matrix_output and out.get("similarity_matrix"):
        try:
            with open(args.similarity_matrix_output, "w", encoding="utf-8") as fsm:
                json.dump(out["similarity_matrix"], fsm, ensure_ascii=False, indent=2)
        except Exception:
            pass
    if args.file_speaker_report and best_report:
        try:
            with open(args.file_speaker_report, "w", encoding="utf-8") as fr:
                json.dump({
                    "file_speaker_best": best_report,
                    "similarity_stats": sim_stats,
                    "suggested_threshold": suggested_threshold
                }, fr, ensure_ascii=False, indent=2)
        except Exception:
            pass
    print(json.dumps(summary, ensure_ascii=False))


if __name__ == "__main__":
    main()
