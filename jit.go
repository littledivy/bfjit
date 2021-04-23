package main

import (
	"encoding/binary"
	"io/ioutil"
	"os"
	"reflect"
	"syscall"
	"unsafe"
)

type Jit struct {
	inst []uint8
}

func (m *Jit) Emit(inst []uint8) {
	m.inst = append(m.inst, inst...)
}

func (m *Jit) Emit32(inst uint32) {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, inst)
	m.Emit(bytes)
}

func (m *Jit) Emit64(inst uint64) {
	m.Emit32(uint32(inst & 0xFFFFFFFF))
	m.Emit32(uint32((inst >> 32) & 0xFFFFFFFF))
}

func (m *Jit) Replace32(offset int, inst uint32) {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, inst)
	for i, b := range bytes {
		m.inst[offset+i] = b
	}
}

func (m *Jit) Compile(code string) []uint8 {
	mem := make([]uint8, 90000)
	stack := make([]int, 0)
	m.Emit([]uint8{0x49, 0xBD})
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&mem))
	m.Emit64(uint64(hdr.Data))
	for _, ch := range code {
		switch ch {
		case '+':
			m.Emit([]uint8{0x41, 0x80, 0x45, 0x00, 0x01})
		case '-':
			m.Emit([]uint8{0x41, 0x80, 0x6D, 0x00, 0x01})
		case '>':
			m.Emit([]uint8{0x49, 0xFF, 0xC5})
		case '<':
			m.Emit([]uint8{0x49, 0xFF, 0xcD})
		case '.':
			m.Emit([]uint8{
				0x48, 0xC7, 0xC0, 0x01, 0x00, 0x00, 0x00,
				0x48, 0xC7, 0xC7, 0x01, 0x00, 0x00, 0x00,
				0x4C, 0x89, 0xEE,
				0x48, 0xC7, 0xC2, 0x01, 0x00, 0x00, 0x00,
				0x0F, 0x05,
			})
		case ',':
			m.Emit([]uint8{
				0x48, 0xC7, 0xC0, 0x00, 0x00, 0x00, 0x00,
				0x48, 0xC7, 0xC7, 0x00, 0x00, 0x00, 0x00,
				0x4C, 0x89, 0xEE,
				0x48, 0xC7, 0xC2, 0x01, 0x00, 0x00, 0x00,
				0x0F, 0x05,
			})
		case '[':
			m.Emit([]uint8{0x41, 0x80, 0x7d, 0x00, 0x00})
			stack = append(stack, len(m.inst))
			m.Emit([]uint8{0x0F, 0x84})
			m.Emit32(0)
		case ']':
			offset := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			m.Emit([]uint8{0x41, 0x80, 0x7d, 0x00, 0x00})

			jmpb_from := len(m.inst) + 6
			jmpb_to := offset + 6
			rel_offset := relative_offset(jmpb_from, jmpb_to)

			m.Emit([]uint8{0x0F, 0x85})
			m.Emit32(uint32(rel_offset))

			jmpf_from := offset + 6
			jmpf_to := len(m.inst)
			rel_offset_forward := relative_offset(jmpf_from, jmpf_to)

			m.Replace32(offset+2, uint32(rel_offset_forward))
		}
	}

	m.Emit([]uint8{0xC3})

	return m.inst
}

func relative_offset(from, to int) int {
	if to > from {

		return to - from
	} else {
		return ^(from - to) + 1
	}
}

func Bf(source string) {
	jit := Jit{inst: []uint8{}}
	code := jit.Compile(source)

	data, _ := syscall.Mmap(-1, 0, len(code), syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC, syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS)
	for i, b := range code {
		data[i] = b
	}
	type execFunc func()
	unsafeFunc := (uintptr)(unsafe.Pointer(&data))
	f := *(*execFunc)(unsafe.Pointer(&unsafeFunc))
	f()
}

func main() {
	sourceFile := os.Args[1:][0]
	dat, _ := ioutil.ReadFile(sourceFile)
	Bf(string(dat))
}
