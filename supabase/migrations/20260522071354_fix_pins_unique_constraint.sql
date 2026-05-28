-- Repair: the existing `pins` table carried a unique constraint on
-- (component_id, label). That is wrong for the workbench catalogue — an STM32
-- Blue Pill legitimately has several pins sharing a label ("G", "3V3").
--
-- Uniqueness belongs on the machine identifier `name` ("L18_G", "L19_G"),
-- which is distinct per physical pin.

alter table public.pins drop constraint if exists pins_component_id_label_key;
alter table public.pins drop constraint if exists pins_label_key;

create unique index if not exists pins_component_name_key
    on public.pins (component_id, name);
