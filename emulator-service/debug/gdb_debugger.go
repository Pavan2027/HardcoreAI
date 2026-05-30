package debug

import (
	"fmt"
)

type GDBDebugger struct {
	mi    *GDBMI
	state DebugState
}

func (g *GDBDebugger) Connect() error {

	mi, err := NewGDBMI(
		"./Blinky/.pio/build/genericSTM32F405RG/firmware.elf",
	)

	if err != nil {
		return err
	}

	err = mi.Connect(
		"127.0.0.1",
		3333,
	)

	if err != nil {
		return err
	}

	g.mi = mi
	g.state = StateHalted

	return nil
}

func (g *GDBDebugger) Disconnect() error {

	if g.mi == nil {
		return nil
	}

	return g.mi.Close()
}

func (g *GDBDebugger) Halt() error {

	err := g.mi.Halt()

	if err != nil {
		return err
	}

	g.state = StateHalted

	return nil
}

func (g *GDBDebugger) Continue() error {

	err := g.mi.Continue()

	if err != nil {
		return err
	}

	g.state = StateRunning

	return nil
}

func (g *GDBDebugger) Step() error {

	err := g.mi.StepInstruction()

	if err != nil {
		return err
	}

	g.state = StateHalted

	return nil
}

func (g *GDBDebugger) Reset() error {

	err := g.mi.Send(
		"monitor reset halt",
	)

	if err != nil {
		return err
	}

	_, err = g.mi.Read()

	return err
}

func (g *GDBDebugger) ReadRegisters() (*Registers, error) {

	lines, err := g.mi.ReadRegisters()

	if err != nil {
		return nil, err
	}

	regs := ParseRegisters(lines)

	return regs, nil
}

func (g *GDBDebugger) ReadMemory(
	addr uint32,
	size uint32,
) ([]byte, error) {

	lines, err := g.mi.ReadMemoryWord(
		addr,
	)

	if err != nil {
		return nil, err
	}

	data := []byte{}

	for _, line := range lines {
		data = append(
			data,
			[]byte(line)...,
		)
	}

	return data, nil
}

func (g *GDBDebugger) WriteMemory(
	addr uint32,
	data []byte,
) error {

	// TODO
	return nil
}

func (g *GDBDebugger) SetBreakpoint(
	addr uint32,
) error {

	location := fmt.Sprintf(
		"*0x%08X",
		addr,
	)

	return g.mi.SetBreakpoint(
		location,
	)
}

func (g *GDBDebugger) RemoveBreakpoint(
	addr uint32,
) error {

	// TODO
	return nil
}

func (g *GDBDebugger) GetState() DebugState {
	return g.state
}
