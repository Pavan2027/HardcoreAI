-- Migration: Add user isolation and auth settings
-- Alters the projects.user_id to uuid and links it to auth.users (Supabase Auth).
-- Sets up Row Level Security (RLS) and policies.

-- 1. Alter projects.user_id to uuid
alter table public.projects 
  alter column user_id type uuid using (
    case 
      when user_id is null then null 
      else user_id::text::uuid 
    end
  );

-- 2. Link to auth.users
alter table public.projects
  add constraint fk_projects_user_id
  foreign key (user_id)
  references auth.users (id)
  on delete cascade;

-- 3. Enable Row Level Security (RLS)
alter table public.projects enable row level security;
alter table public.project_components enable row level security;
alter table public.project_connections enable row level security;
alter table public.code_files enable row level security;

-- 4. Create RLS Policies

-- Policies for projects
create policy "Users can manage their own projects"
  on public.projects
  for all
  to authenticated
  using (auth.uid() = user_id)
  with check (auth.uid() = user_id);

-- Policies for project_components
create policy "Users can manage components of their own projects"
  on public.project_components
  for all
  to authenticated
  using (
    exists (
      select 1 from public.projects 
      where projects.id = project_components.project_id 
      and projects.user_id = auth.uid()
    )
  )
  with check (
    exists (
      select 1 from public.projects 
      where projects.id = project_components.project_id 
      and projects.user_id = auth.uid()
    )
  );

-- Policies for project_connections
create policy "Users can manage connections of their own projects"
  on public.project_connections
  for all
  to authenticated
  using (
    exists (
      select 1 from public.projects 
      where projects.id = project_connections.project_id 
      and projects.user_id = auth.uid()
    )
  )
  with check (
    exists (
      select 1 from public.projects 
      where projects.id = project_connections.project_id 
      and projects.user_id = auth.uid()
    )
  );

-- Policies for code_files
create policy "Users can manage code files of their own projects"
  on public.code_files
  for all
  to authenticated
  using (
    exists (
      select 1 from public.projects 
      where projects.id = code_files.project_id 
      and projects.user_id = auth.uid()
    )
  )
  with check (
    exists (
      select 1 from public.projects 
      where projects.id = code_files.project_id 
      and projects.user_id = auth.uid()
    )
  );
