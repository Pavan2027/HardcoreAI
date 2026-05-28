-- Seed the component catalogue: the data that used to live hardcoded in
-- backend/main.py (COMPONENTS list). Idempotent - re-running upserts by slug.
--
-- Pin placement coordinates match the visual chip dimensions; chip *styling*
-- (colours, shapes) lives in frontend chip_styles.css keyed by visual_type.

-- ---------------------------------------------------------------------------
-- Components
-- ---------------------------------------------------------------------------

insert into public.components
    (slug, name, library_name, description, is_controller, cpp_class_name, header_file,
     category, visual_type, thumbnail, width, height)
values
    ('stm32-blue-pill', 'STM32 Blue Pill', 'STM32duino', 'STM32F103C8T6 development board with USB end, crystal, reset, and labeled headers.',
     true, 'BluePill', 'stm32f1xx_hal.h', 'Microcontroller', 'blue-pill', 'board', 190, 260),
    ('dc-motor', 'DC Motor', 'Motor', 'Two-terminal brushed DC motor for basic motion projects.',
     false, 'DCMotor', 'Motor.h', 'Actuator', 'motor', 'motor', 150, 110),
    ('l298n-driver', 'L298N Motor Driver', 'L298N', 'Dual H-bridge motor driver with inputs, outputs, and power terminals.',
     false, 'L298N', 'L298N.h', 'Driver', 'driver', 'driver', 190, 140),
    ('led-red', 'Red LED', 'Arduino', 'Simple red LED with anode and cathode pins.',
     false, 'Led', 'Arduino.h', 'Indicator', 'led', 'led', 105, 90),
    ('resistor-220', '220 Ohm Resistor', null, 'Current limiting resistor for LED and signal protection circuits.',
     false, null, null, 'Passive', 'resistor', 'resistor', 130, 70),
    ('battery-9v', '9V Battery', null, 'Portable 9V source for motor and driver experiments.',
     false, null, null, 'Power', 'battery', 'battery', 110, 150)
on conflict (slug) do update set
    name          = excluded.name,
    library_name  = excluded.library_name,
    description   = excluded.description,
    is_controller = excluded.is_controller,
    cpp_class_name = excluded.cpp_class_name,
    header_file   = excluded.header_file,
    category      = excluded.category,
    visual_type   = excluded.visual_type,
    thumbnail     = excluded.thumbnail,
    width         = excluded.width,
    height        = excluded.height;

-- ---------------------------------------------------------------------------
-- Pins. Cleared and re-inserted per component so the seed is reproducible.
-- ---------------------------------------------------------------------------

delete from public.pins
    where component_id in (select id from public.components);

-- STM32 Blue Pill: 20 left + 20 right header pins, generated procedurally.
do $$
declare
    mcu_id bigint;
    left_labels  text[] := array['VB','C13','C14','C15','A0','A1','A2','A3','A4','A5','A6','A7','B0','B1','B10','B11','R','3V3','G','G'];
    right_labels text[] := array['5V','G','3V3','B12','B13','B14','B15','A8','A9','A10','A11','A12','A15','B3','B4','B5','B6','B7','B8','B9'];
    idx int;
    lbl text;
    pin_role text;
begin
    select id into mcu_id from public.components where slug = 'stm32-blue-pill';

    for idx in 0 .. array_length(left_labels, 1) - 1 loop
        lbl := left_labels[idx + 1];
        pin_role := case when lbl in ('VB','3V3','G') then 'power' else 'gpio' end;
        insert into public.pins (component_id, name, label, side, x, y, role, is_input, is_output)
        values (mcu_id, 'L' || idx || '_' || lbl, lbl, 'left', 0, 20 + idx * 11, pin_role,
                pin_role = 'gpio', pin_role = 'gpio');
    end loop;

    for idx in 0 .. array_length(right_labels, 1) - 1 loop
        lbl := right_labels[idx + 1];
        pin_role := case when lbl in ('5V','3V3','G') then 'power' else 'gpio' end;
        insert into public.pins (component_id, name, label, side, x, y, role, is_input, is_output)
        values (mcu_id, 'R' || idx || '_' || lbl, lbl, 'right', 190, 20 + idx * 11, pin_role,
                pin_role = 'gpio', pin_role = 'gpio');
    end loop;
end $$;

-- DC Motor
insert into public.pins (component_id, name, label, side, x, y, role)
select id, 'M_PLUS',  'M+', 'left', 0, 42, 'motor' from public.components where slug = 'dc-motor'
union all
select id, 'M_MINUS', 'M-', 'left', 0, 68, 'motor' from public.components where slug = 'dc-motor';

-- L298N Motor Driver
insert into public.pins (component_id, name, label, side, x, y, role)
select id, 'IN1',  'IN1',  'left',   0,  32, 'input'  from public.components where slug = 'l298n-driver'
union all select id, 'IN2',  'IN2',  'left',   0,  58, 'input'  from public.components where slug = 'l298n-driver'
union all select id, 'ENA',  'ENA',  'left',   0,  84, 'pwm'    from public.components where slug = 'l298n-driver'
union all select id, 'OUT1', 'OUT1', 'right',190,  42, 'output' from public.components where slug = 'l298n-driver'
union all select id, 'OUT2', 'OUT2', 'right',190,  70, 'output' from public.components where slug = 'l298n-driver'
union all select id, '12V',  '12V',  'bottom', 70, 140, 'power'  from public.components where slug = 'l298n-driver'
union all select id, 'GND',  'GND',  'bottom',118, 140, 'power'  from public.components where slug = 'l298n-driver';

-- Red LED
insert into public.pins (component_id, name, label, side, x, y, role)
select id, 'ANODE',   'A', 'left',    0, 45, 'input'  from public.components where slug = 'led-red'
union all
select id, 'CATHODE', 'K', 'right', 105, 45, 'ground' from public.components where slug = 'led-red';

-- 220 Ohm Resistor
insert into public.pins (component_id, name, label, side, x, y, role)
select id, 'A', '1', 'left',    0, 35, 'passive' from public.components where slug = 'resistor-220'
union all
select id, 'B', '2', 'right', 130, 35, 'passive' from public.components where slug = 'resistor-220';

-- 9V Battery
insert into public.pins (component_id, name, label, side, x, y, role)
select id, 'POS', '+', 'top', 42, 0, 'power'  from public.components where slug = 'battery-9v'
union all
select id, 'NEG', '-', 'top', 68, 0, 'ground' from public.components where slug = 'battery-9v';
