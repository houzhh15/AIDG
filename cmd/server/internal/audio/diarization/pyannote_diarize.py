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
import yaml
from pathlib import Path

# =============================================================================
# CRITICAL: Apply torch.load patch BEFORE importing pyannote or lightning
# PyTorch 2.6+ changed default weights_only=True, breaking pyannote models
# =============================================================================
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

# Global variable to hold Pipeline class (lazy loaded)
Pipeline = None


def _ensure_pipeline_imported():
    """Lazily import Pipeline to allow monkey-patching before import."""
    global Pipeline
    if Pipeline is None:
        try:
            from pyannote.audio import Pipeline as _Pipeline
            Pipeline = _Pipeline
            
            # Re-apply patches after import (some modules cache references)
            import pyannote.audio.core.model
            try:
                pyannote.audio.core.model.pl_load = _patched_cloud_io_load
            except:
                pass
        except Exception as e:
            print(json.dumps({"segments": [], "error": f"pyannote not available: {e}"}))
            sys.exit(0)
    return Pipeline


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
    
    # Set HF token via environment variable for pyannote.audio Pipeline
    # Pipeline.from_pretrained() doesn't accept token parameter directly
    # For huggingface_hub compatibility, also set the old env var names
    if args.hf_token:
        os.environ["HF_TOKEN"] = args.hf_token
        os.environ["HUGGING_FACE_HUB_TOKEN"] = args.hf_token
        os.environ["HUGGINGFACE_TOKEN"] = args.hf_token  # Legacy support

    try:
        # Ensure input is WAV mono 16k for robust decoding
        wav_path = ensure_wav_mono_16k(args.input)

        # Resolve pipeline (offline/local or online)
        pipeline = None
        if is_local_pipeline:
            # Load from a local directory or YAML file directly, no network
            _ensure_pipeline_imported()
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
            # Offline load: use local snapshot paths to avoid network calls
            pipeline = load_pipeline_offline(args.pipeline, args.cache_dir)
        else:
            # Online load; HF hub will reuse cache if available
            # pyannote.audio 3.1.1 doesn't accept token parameter
            # Must use environment variables (already set above if args.hf_token exists)
            _ensure_pipeline_imported()
            pipeline = Pipeline.from_pretrained(args.pipeline, cache_dir=args.cache_dir)
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


def load_pipeline_offline(pipeline_name: str, cache_dir: str = None):
    """
    Load pyannote pipeline in fully offline mode by resolving all model paths locally.
    
    This function works around pyannote.audio's internal network calls by:
    1. Monkey-patching hf_hub_download and torch.load BEFORE importing pyannote
    2. Pre-downloading all required models using snapshot_download with local_files_only=True
    3. Setting up environment to force offline mode
    4. Overriding cache_dir to ensure pyannote uses our configured cache location
    """
    global Pipeline
    
    import huggingface_hub
    import huggingface_hub.file_download
    import torch
    
    # Ensure HF_HUB_OFFLINE is set
    os.environ["HF_HUB_OFFLINE"] = "1"
    
    # Determine the effective cache directory
    # If not specified, use the environment variable or default
    effective_cache_dir = cache_dir or os.environ.get("HF_HUB_CACHE") or os.environ.get("HF_HOME")
    if effective_cache_dir and not effective_cache_dir.endswith("/hub"):
        # Normalize to hub directory path
        hub_cache_dir = os.path.join(effective_cache_dir, "hub") if os.path.isdir(os.path.join(effective_cache_dir, "hub")) else effective_cache_dir
    else:
        hub_cache_dir = effective_cache_dir
    
    # Store original functions BEFORE any imports that might use them
    original_hf_hub_download = huggingface_hub.file_download.hf_hub_download
    original_torch_load = torch.load
    
    def patched_hf_hub_download(*args, **kwargs):
        kwargs['local_files_only'] = True
        # CRITICAL: Override cache_dir to use our configured cache location
        if hub_cache_dir:
            kwargs['cache_dir'] = hub_cache_dir
        # Remove auth token parameters to avoid network validation
        kwargs.pop('use_auth_token', None)
        kwargs.pop('token', None)
        return original_hf_hub_download(*args, **kwargs)
    
    def patched_torch_load(f, *args, **kwargs):
        # Force weights_only=False for older checkpoints (PyTorch 2.6+ compatibility)
        # This is required for pyannote models which contain non-standard globals
        if 'weights_only' not in kwargs or kwargs.get('weights_only') is None:
            kwargs['weights_only'] = False
        return original_torch_load(f, *args, **kwargs)
    
    # Apply patches BEFORE importing pyannote or its dependencies
    huggingface_hub.hf_hub_download = patched_hf_hub_download
    huggingface_hub.file_download.hf_hub_download = patched_hf_hub_download
    torch.load = patched_torch_load
    
    # CRITICAL: Patch lightning_fabric BEFORE importing pyannote
    # pyannote uses lightning_fabric.utilities.cloud_io._load which calls torch.load
    try:
        import lightning_fabric.utilities.cloud_io as cloud_io
        original_cloud_io_load = cloud_io._load
        
        def patched_cloud_io_load(path_or_url, map_location=None, weights_only=None):
            # Force weights_only=False for pyannote model compatibility
            if weights_only is None:
                weights_only = False
            return original_cloud_io_load(path_or_url, map_location=map_location, weights_only=weights_only)
        
        cloud_io._load = patched_cloud_io_load
    except Exception as e:
        print(f"Warning: Could not patch lightning_fabric: {e}", file=sys.stderr)
    
    try:
        # Import snapshot_download for verification (after patching)
        from huggingface_hub import snapshot_download
        
        # Get the main pipeline's local cache path to verify it exists
        local_dir = snapshot_download(pipeline_name, local_files_only=True, cache_dir=hub_cache_dir)
        cfg_path = os.path.join(local_dir, "config.yaml")
        
        if not os.path.isfile(cfg_path):
            raise RuntimeError(f"Pipeline config not found: {cfg_path}")
        
        # Read and parse the config to find sub-models
        with open(cfg_path, 'r') as f:
            config = yaml.safe_load(f)
        
        # Pre-verify all sub-models are cached (this will raise if any are missing)
        params = config.get('pipeline', {}).get('params', {})
        for key, value in params.items():
            if isinstance(value, str) and '/' in value and not value.startswith('/'):
                try:
                    sub_model_dir = snapshot_download(value, local_files_only=True, cache_dir=hub_cache_dir)
                    if not os.path.isdir(sub_model_dir):
                        raise RuntimeError(f"Sub-model not cached: {value}")
                except Exception as e:
                    raise RuntimeError(f"Sub-model {value} not cached. Please download it first: {e}")
        
        # NOW import pyannote (with patches in place)
        # Clear any previously imported pyannote modules to ensure fresh import with patches
        import sys
        pyannote_modules = [k for k in sys.modules.keys() if k.startswith('pyannote')]
        for mod in pyannote_modules:
            del sys.modules[mod]
        
        # Do NOT clear lightning modules - we've already patched them above
        # and we want to keep our patches in place
        
        from pyannote.audio import Pipeline as _Pipeline
        Pipeline = _Pipeline
        
        # After importing, also patch the module-level references
        import pyannote.audio.core.pipeline
        import pyannote.audio.core.model
        pyannote.audio.core.pipeline.hf_hub_download = patched_hf_hub_download
        pyannote.audio.core.model.hf_hub_download = patched_hf_hub_download
        
        # Re-apply patches to lightning_fabric after pyannote import
        # (pyannote might have re-imported with different references)
        try:
            import lightning_fabric.utilities.cloud_io as cloud_io_module
            cloud_io_module._load = patched_cloud_io_load
            cloud_io_module.torch.load = patched_torch_load
        except Exception:
            pass
        
        # Also patch pyannote.audio.core.model's pl_load reference directly
        try:
            pyannote.audio.core.model.pl_load = patched_cloud_io_load
        except Exception:
            pass
        
        # Also patch torch module directly for any remaining references
        torch.load = patched_torch_load
        
        # Patch pytorch_lightning if it was imported
        try:
            import pytorch_lightning.utilities.cloud_io
            pytorch_lightning.utilities.cloud_io.torch.load = patched_torch_load
        except:
            pass
        
        pipeline = Pipeline.from_pretrained(pipeline_name, cache_dir=hub_cache_dir)
        return pipeline
        
    finally:
        # Restore original functions
        huggingface_hub.hf_hub_download = original_hf_hub_download
        huggingface_hub.file_download.hf_hub_download = original_hf_hub_download
        torch.load = original_torch_load


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
