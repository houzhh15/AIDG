"""
NLP Service for AI Dev Gov
Provides text embedding API using text2vec-base-chinese model
"""
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field, validator
from sentence_transformers import SentenceTransformer
from typing import List, Optional
import numpy as np
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI(
    title="NLP Service",
    description="Text embedding service for semantic similarity",
    version="1.0.0"
)

# Configure CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # In production, specify actual origins
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Global model instance
model: Optional[SentenceTransformer] = None


class EmbedRequest(BaseModel):
    """Request model for embedding endpoint"""
    texts: List[str] = Field(..., description="List of texts to embed", min_items=1, max_items=100)
    model: Optional[str] = Field(default="text2vec-base-chinese", description="Model name")
    
    @validator('texts')
    def validate_texts(cls, v):
        if not v:
            raise ValueError("texts cannot be empty")
        if len(v) > 100:
            raise ValueError("Batch size exceeds 100")
        return v


class EmbedResponse(BaseModel):
    """Response model for embedding endpoint"""
    embeddings: List[List[float]] = Field(..., description="List of embedding vectors")
    model: str = Field(..., description="Model name used")
    dim: int = Field(..., description="Dimension of embeddings")


@app.on_event("startup")
async def startup_event():
    """Load model on startup"""
    global model
    try:
        logger.info("Loading text2vec-base-chinese model...")
        model = SentenceTransformer('shibing624/text2vec-base-chinese')
        logger.info("Model loaded successfully")
    except Exception as e:
        logger.error(f"Failed to load model: {e}")
        raise


@app.on_event("shutdown")
async def shutdown_event():
    """Cleanup on shutdown"""
    logger.info("Shutting down NLP service")


@app.get("/")
async def root():
    """Health check endpoint"""
    return {"status": "ok", "service": "nlp-service", "version": "1.0.0"}


@app.get("/health")
async def health():
    """Detailed health check"""
    return {
        "status": "healthy",
        "model_loaded": model is not None,
        "model_name": "text2vec-base-chinese"
    }


@app.post("/nlp/embed", response_model=EmbedResponse)
async def embed_texts(request: EmbedRequest):
    """
    Embed texts using text2vec-base-chinese model
    
    Args:
        request: EmbedRequest containing texts to embed
        
    Returns:
        EmbedResponse with embeddings, model name and dimension
        
    Raises:
        HTTPException: If embedding fails
    """
    if model is None:
        raise HTTPException(status_code=503, detail="Model not loaded")
    
    try:
        logger.info(f"Embedding {len(request.texts)} texts...")
        
        # Encode texts with batching
        embeddings = model.encode(
            request.texts,
            batch_size=32,
            show_progress_bar=False,
            convert_to_numpy=True,
            normalize_embeddings=False
        )
        
        # Convert to list of lists
        embeddings_list = embeddings.tolist()
        
        logger.info(f"Successfully embedded {len(embeddings_list)} texts")
        
        return EmbedResponse(
            embeddings=embeddings_list,
            model=request.model or "text2vec-base-chinese",
            dim=embeddings.shape[1]
        )
        
    except Exception as e:
        logger.error(f"Embedding failed: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Embedding failed: {str(e)}")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=5000, log_level="info")
