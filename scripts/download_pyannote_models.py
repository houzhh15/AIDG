#!/usr/bin/env python3
"""
Download PyAnnote models for offline use.

This script downloads all required models for pyannote speaker diarization
to the specified cache directory, enabling fully offline operation.

Usage:
    python3 download_pyannote_models.py [--cache_dir /path/to/cache] [--token HF_TOKEN]

Required models:
    - pyannote/speaker-diarization-3.1 (pipeline config)
    - pyannote/segmentation-3.0 (segmentation model)
    - pyannote/wespeaker-voxceleb-resnet34-LM (embedding model)

Note: These models require accepting the license agreement on HuggingFace.
      Visit https://huggingface.co/pyannote/speaker-diarization-3.1 to accept.
"""

import argparse
import os
import sys
from pathlib import Path


# Models required for speaker-diarization-3.1 pipeline
REQUIRED_MODELS = [
    "pyannote/speaker-diarization-3.1",  # Main pipeline config
    "pyannote/segmentation-3.0",          # Segmentation model
    "pyannote/wespeaker-voxceleb-resnet34-LM",  # Speaker embedding model (used by diarization)
    "pyannote/embedding",                 # Speaker embedding model (used by generate_speaker_embeddings.py)
]


def check_model_cached(repo_id: str, cache_dir: str) -> bool:
    """Check if a model is already cached."""
    from huggingface_hub import try_to_load_from_cache
    from huggingface_hub.constants import HUGGINGFACE_HUB_CACHE
    
    effective_cache = cache_dir or HUGGINGFACE_HUB_CACHE
    
    # Check for config.yaml (all pyannote models have this)
    result = try_to_load_from_cache(
        repo_id=repo_id,
        filename="config.yaml",
        cache_dir=effective_cache
    )
    
    if result is None or result == "NOT_FOUND":
        return False
    
    # For models with weights, also check pytorch_model.bin
    if repo_id != "pyannote/speaker-diarization-3.1":
        weights_result = try_to_load_from_cache(
            repo_id=repo_id,
            filename="pytorch_model.bin",
            cache_dir=effective_cache
        )
        if weights_result is None or weights_result == "NOT_FOUND":
            return False
    
    return True


def download_model(repo_id: str, cache_dir: str, token: str = None) -> bool:
    """Download a model to the cache directory."""
    from huggingface_hub import snapshot_download
    
    print(f"  Downloading {repo_id}...")
    try:
        local_dir = snapshot_download(
            repo_id=repo_id,
            cache_dir=cache_dir,
            token=token,
            local_files_only=False,
        )
        print(f"  ✓ Downloaded to: {local_dir}")
        return True
    except Exception as e:
        print(f"  ✗ Failed to download {repo_id}: {e}")
        return False


def verify_model(repo_id: str, cache_dir: str) -> bool:
    """Verify a model can be loaded offline."""
    from huggingface_hub import snapshot_download
    
    try:
        local_dir = snapshot_download(
            repo_id=repo_id,
            cache_dir=cache_dir,
            local_files_only=True,
        )
        
        # Check required files exist
        config_path = os.path.join(local_dir, "config.yaml")
        if not os.path.isfile(config_path):
            return False
        
        # Check weights for model repos
        if repo_id != "pyannote/speaker-diarization-3.1":
            weights_path = os.path.join(local_dir, "pytorch_model.bin")
            if not os.path.isfile(weights_path):
                return False
        
        return True
    except Exception:
        return False


def get_cache_size(cache_dir: str) -> str:
    """Get human-readable cache size."""
    total_size = 0
    hub_dir = os.path.join(cache_dir, "hub") if not cache_dir.endswith("hub") else cache_dir
    
    if os.path.isdir(hub_dir):
        for dirpath, dirnames, filenames in os.walk(hub_dir):
            for f in filenames:
                fp = os.path.join(dirpath, f)
                if os.path.isfile(fp):
                    total_size += os.path.getsize(fp)
    
    # Convert to human readable
    for unit in ['B', 'KB', 'MB', 'GB']:
        if total_size < 1024:
            return f"{total_size:.1f} {unit}"
        total_size /= 1024
    return f"{total_size:.1f} TB"


def main():
    parser = argparse.ArgumentParser(
        description="Download PyAnnote models for offline speaker diarization"
    )
    parser.add_argument(
        "--cache_dir",
        default=os.environ.get("HF_HUB_CACHE", os.path.expanduser("~/.cache/huggingface/hub")),
        help="Cache directory for models (default: $HF_HUB_CACHE or ~/.cache/huggingface/hub)"
    )
    parser.add_argument(
        "--token",
        default=os.environ.get("HF_TOKEN") or os.environ.get("HUGGINGFACE_TOKEN"),
        help="HuggingFace token (required for gated models)"
    )
    parser.add_argument(
        "--force",
        action="store_true",
        help="Force re-download even if models exist"
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Only check if models are cached, don't download"
    )
    args = parser.parse_args()
    
    # Normalize cache directory
    cache_dir = args.cache_dir
    if not cache_dir.endswith("hub"):
        cache_dir = os.path.join(cache_dir, "hub")
    
    print("=" * 60)
    print("PyAnnote Model Downloader")
    print("=" * 60)
    print(f"Cache directory: {cache_dir}")
    print(f"HF Token: {'***' + args.token[-4:] if args.token else 'Not provided'}")
    print()
    
    # Import huggingface_hub
    try:
        import huggingface_hub
        print(f"huggingface_hub version: {huggingface_hub.__version__}")
    except ImportError:
        print("Error: huggingface_hub not installed. Run: pip install huggingface_hub")
        sys.exit(1)
    
    # Create cache directory
    os.makedirs(cache_dir, exist_ok=True)
    
    print()
    print("Checking required models:")
    print("-" * 40)
    
    models_status = {}
    all_cached = True
    
    for repo_id in REQUIRED_MODELS:
        cached = check_model_cached(repo_id, cache_dir)
        models_status[repo_id] = cached
        status = "✓ Cached" if cached else "✗ Missing"
        print(f"  {repo_id}: {status}")
        if not cached:
            all_cached = False
    
    print()
    
    if args.check:
        # Check-only mode
        if all_cached:
            print("✓ All models are cached and ready for offline use!")
            print(f"  Cache size: {get_cache_size(cache_dir)}")
            sys.exit(0)
        else:
            print("✗ Some models are missing. Run without --check to download.")
            sys.exit(1)
    
    if all_cached and not args.force:
        print("✓ All models are already cached!")
        print(f"  Cache size: {get_cache_size(cache_dir)}")
        print()
        print("Use --force to re-download models.")
        sys.exit(0)
    
    # Check for token (required for pyannote models)
    if not args.token:
        print("⚠ Warning: No HuggingFace token provided.")
        print("  PyAnnote models are gated and require authentication.")
        print("  Please provide a token via --token or HF_TOKEN environment variable.")
        print()
        print("  To get a token:")
        print("  1. Create account at https://huggingface.co")
        print("  2. Accept license at https://huggingface.co/pyannote/speaker-diarization-3.1")
        print("  3. Get token from https://huggingface.co/settings/tokens")
        print()
        response = input("Continue without token? [y/N]: ")
        if response.lower() != 'y':
            sys.exit(1)
    
    # Download missing models
    print("Downloading models:")
    print("-" * 40)
    
    success = True
    for repo_id in REQUIRED_MODELS:
        if models_status[repo_id] and not args.force:
            print(f"  {repo_id}: Skipping (already cached)")
            continue
        
        if not download_model(repo_id, cache_dir, args.token):
            success = False
    
    print()
    
    # Verify downloads
    print("Verifying models:")
    print("-" * 40)
    
    all_verified = True
    for repo_id in REQUIRED_MODELS:
        verified = verify_model(repo_id, cache_dir)
        status = "✓ OK" if verified else "✗ Failed"
        print(f"  {repo_id}: {status}")
        if not verified:
            all_verified = False
    
    print()
    
    if all_verified:
        print("=" * 60)
        print("✓ All models downloaded and verified successfully!")
        print(f"  Cache size: {get_cache_size(cache_dir)}")
        print()
        print("You can now use PyAnnote in offline mode.")
        print(f"Set HF_HUB_CACHE={cache_dir}")
        print("=" * 60)
        sys.exit(0)
    else:
        print("=" * 60)
        print("✗ Some models failed verification.")
        print("  Please check your HuggingFace token and try again.")
        print("=" * 60)
        sys.exit(1)


if __name__ == "__main__":
    main()
