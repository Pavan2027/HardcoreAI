create table if not exists project_attachments (
    id uuid primary key,
    project_id text not null,
    filename text not null,
    source_path text not null,
    mime_type text,
    created_at timestamptz not null default now()
);

create table if not exists agent_run_attachments (
    id uuid primary key,
    run_id text not null,
    project_attachment_id uuid not null references project_attachments(id) on delete cascade,
    indexed_into_rag boolean not null default false,
    created_at timestamptz not null default now()
);

create index if not exists idx_project_attachments_project_id
    on project_attachments(project_id);

create index if not exists idx_agent_run_attachments_run_id
    on agent_run_attachments(run_id);
