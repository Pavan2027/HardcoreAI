alter table public.project_connections
  add column if not exists label text,
  add column if not exists color text;
