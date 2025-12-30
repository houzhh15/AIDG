#!/usr/bin/env python3
"""
Minimal pyannote-based diarization helper.

Usage:
        python3 pyannote_diarize.py --input <audio_path> [--hf_token <token>] [--pipeline <hf_pipeline_id_or_local_dir>] [--offline] [--cache_dir <dir>]

Outputs JSON to stdout:
    { "segments": [ {"start": 0.00, "end": 2.34, "speaker": "SPK00"}, ... ] }

Notes:
- Requires: pip install pyannote.audio torch librosa soundfile huggingface_hub
- For Hugging Face hosted models, a token is usually required. If --offline is specified or --pipeline points to a local directory, no network is used and token is not required.
- If HF token is not passed via --hf_token, will read from environment HUGGINGFACE_TOKEN.
- You can also set HF_HUB_OFFLINE=1 to force offline behavior.
- For long files, diarization runs offline and may take time.
"""
import argparse
import warnings
import json
import os
import sys
import shutil
import tempfile
import subprocess
import platform
from pathlib import Path

# Optional: only import pyannote when executed to avoid failing when not installed
try:
    from pyannote.audio import Pipeline
except Exception as e:
    print(json.dumps({"segments": [], "error": f"pyannote not available: {e}"}))
    sys.exit(0)


def main():
    # Reduce noisy warnings that may pollute stdout
    warnings.filterwarnings("ignore")
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--hf_token", default=os.getenv("HUGGINGFACE_TOKEN"))
    parser.add_argument("--pipeline", default="pyannote/speaker-diarization-3.1", help="HF repo id or local directory")
    parser.add_argument("--num_speakers", type=int, default=None)
    parser.add_argument("--min_speakers", type=int, default=None)
    parser.add_argument("--max_speakers", type=int, default=None)
    parser.add_argument("--device", default="auto", choices=["auto", "cpu", "mps", "cuda"])
    parser.add_argument("--offline", action="store_true", help="Use cached/local models without network")
    parser.add_argument("--cache_dir", default=None, help="Hugging Face cache directory")
    args = parser.parse_args()

    if not os.path.exists(args.input):
        print(json.dumps({"segments": [], "error": "input audio not found"}))
        return

    # If using offline mode or local pipeline path, token is not required
    is_local_pipeline = os.path.isdir(args.pipeline) or (os.path.isfile(args.pipeline) and args.pipeline.lower().endswith((".yml", ".yaml")))
    if args.offline:
        # Force offline mode - do not use setdefault to ensure it's set
        os.environ["HF_HUB_OFFLINE"] = "1"
    if not args.hf_token and not args.offline and not is_local_pipeline:
        print(json.dumps({"segments": [], "error": "missing HF token"}))
        return

    try:
        # Ensure input is WAV mono 16k for robust decoding
        wav_path = ensure_wav_mono_16k(args.input)

        # Resolve pipeline (offline/local or online)
        pipeline = None
        if is_local_pipeline:
            # Load from a local directory or YAML file directly, no network
            if os.path.isdir(args.pipeline):
                cfg = os.path.join(args.pipeline, "config.yaml")
                if not os.path.isfile(cfg):
                    # Fallback: pick the first *.yml|*.yaml under the directory
                    ymls = [str(p) for p in Path(args.pipeline).glob("*.yml")] + [str(p) for p in Path(args.pipeline).glob("*.yaml")]
                    if not ymls:
                        raise RuntimeError(f"local pipeline directory missing YAML config: {args.pipeline}")
                    cfg = sorted(ymls)[0]
                pipeline = Pipeline.from_pretrained(cfg)
            else:
                # YAML file path provided
                pipeline = Pipeline.from_pretrained(args.pipeline)
        elif args.offline:
            # Offline load via snapshot_download to resolve local cache path
            try:
                from huggingface_hub import snapshot_download
            except Exception as e:
                raise RuntimeError(f"offline mode requires huggingface_hub installed: {e}")
            # In offline mode, don't use token to avoid any network calls
            local_dir = snapshot_download(args.pipeline, local_files_only=True, token=None, cache_dir=args.cache_dir)
            cfg = os.path.join(local_dir, "config.yaml")
            if not os.path.isfile(cfg):
                # Fallback: pick the first *.yml|*.yaml under the snapshot
                ymls = [str(p) for p in Path(local_dir).glob("*.yml")] + [str(p) for p in Path(local_dir).glob("*.yaml")]
                if not ymls:
                    raise RuntimeError(f"cached snapshot missing YAML config: {local_dir}")
                cfg = sorted(ymls)[0]
            pipeline = Pipeline.from_pretrained(cfg)
        else:
            # Online load; HF hub will reuse cache if available
            pipeline = Pipeline.from_pretrained(args.pipeline, token=args.hf_token, cache_dir=args.cache_dir)
        # Select device
        device = resolve_device(args.device)
        try:
            pipeline.to(device)
        except Exception:
            # Some pipeline components may not support .to(); ignore silently
            pass
        # Newer pyannote pipelines accept guidance on number of speakers via protocol kwargs
        kwargs = {}
        if args.num_speakers is not None:
            kwargs["num_speakers"] = args.num_speakers
        if args.min_speakers is not None:
            kwargs["min_speakers"] = args.min_speakers
        if args.max_speakers is not None:
            kwargs["max_speakers"] = args.max_speakers

        diarization = pipeline(wav_path, **kwargs)

        # pyannote timeline to segments
        speakers = sorted({turn[2] for turn in diarization.itertracks(yield_label=True)})
        spk_index = {spk: f"SPK{idx:02d}" for idx, spk in enumerate(speakers)}

        out = []
        for segment, _, label in diarization.itertracks(yield_label=True):
            out.append({
                "start": float(segment.start),
                "end": float(segment.end),
                "speaker": spk_index.get(label, str(label))
            })

        print(json.dumps({"segments": out}, ensure_ascii=False))
    except Exception as e:
        print(json.dumps({"segments": [], "error": str(e)}))


def resolve_device(pref: str) -> str:
    """Resolve device string. auto -> mps on Apple Silicon (if available), else cuda, else cpu."""
    if pref in {"cpu", "mps", "cuda"}:
        return pref
    # auto
    try:
        import torch
        if platform.system() == "Darwin" and torch.backends.mps.is_available():
            return "mps"
        if torch.cuda.is_available():
            return "cuda"
    except Exception:
        pass
    return "cpu"


def ensure_wav_mono_16k(src_path: str) -> str:
    """Convert source audio to a temporary WAV 16k mono file.
    Prefer ffmpeg; fallback to librosa+soundfile.
    Returns path to the converted WAV file.
    """
    tmpdir = tempfile.mkdtemp(prefix="pyannote_wav_")
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
            # Fallback to librosa if ffmpeg fails
            print(f"ffmpeg convert failed: {e}", file=sys.stderr)

    # Fallback using librosa+soundfile
    try:
        import librosa
        import soundfile as sf
        y, sr = librosa.load(src_path, sr=16000, mono=True)
        sf.write(dst_path, y, 16000)
        return dst_path
    except Exception as e:
        raise RuntimeError(f"fallback convert failed: {e}")


if __name__ == "__main__":
    main()
