from __future__ import annotations

import shutil
from pathlib import Path
from uuid import uuid4

from fastapi import APIRouter, File, Form, HTTPException, UploadFile

from .rag_service import RAGConfig, RAGService

router = APIRouter(prefix="/api", tags=["rag"])


def _persist_upload(upload_dir: Path, upload: UploadFile, max_bytes: int) -> Path:
    upload_dir.mkdir(parents=True, exist_ok=True)
    safe_name = Path(upload.filename or f"{uuid4()}.pdf").name
    target = upload_dir / safe_name
    with target.open("wb") as temp_file:
        bytes_written = 0
        while True:
            chunk = upload.file.read(1024 * 1024)
            if not chunk:
                break
            bytes_written += len(chunk)
            if bytes_written > max_bytes:
                raise HTTPException(status_code=413, detail=f"{upload.filename} exceeds upload limit")
            temp_file.write(chunk)
    return target


@router.get("/health")
async def health() -> dict[str, object]:
    service = RAGService()
    return service.health_check()


@router.post("/projects/{project_id}/agent/solve")
async def solve_project(
    project_id: str,
    problem: str = Form(...),
    llm_provider: str = Form(...),
    chip_family: str = Form(""),
    documents: list[UploadFile] | None = File(default=None),
) -> dict[str, object]:
    config = RAGConfig.from_env()
    service = RAGService(config)
    upload_dir = config.upload_dir / project_id
    temp_files: list[Path] = []

    try:
        for upload in documents or []:
            extension = Path(upload.filename or "").suffix.lower()
            if extension not in config.allowed_extensions:
                raise HTTPException(status_code=400, detail=f"Unsupported file type: {upload.filename}")
            temp_file = _persist_upload(upload_dir, upload, config.max_upload_size_mb * 1024 * 1024)
            temp_files.append(temp_file)

        copied_docs: list[str] = []
        if temp_files:
            copied_docs = service.stage_documents(temp_files)
            service.ingest()

        query_result = service.query(problem, chip_family=chip_family)
        run_id = str(uuid4())

        return {
            "run_id": run_id,
            "project_id": project_id,
            "llm_provider": llm_provider,
            "chip_family": chip_family,
            "documents_indexed": copied_docs,
            "rag_context": query_result["context"],
            "raw_stdout": query_result["stdout"],
        }
    except HTTPException:
        raise
    except RuntimeError as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc
    finally:
        for temp_file in temp_files:
            temp_file.unlink(missing_ok=True)
        if upload_dir.exists():
            shutil.rmtree(upload_dir, ignore_errors=True)
