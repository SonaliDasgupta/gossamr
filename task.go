package gossamr

import (
	"fmt"
	"io"
	"log"
	"reflect"
)

type Phase uint8

const (
	MapPhase Phase = iota
	CombinePhase
	ReducePhase
)

func GetPhase(name string) (Phase, error) {
	switch name {
	default:
		return 0, fmt.Errorf("Unknown phase %s", name)
	case "":
		return 0, fmt.Errorf("Missing phase")
	case "map":
		return MapPhase, nil
	case "combine":
		return CombinePhase, nil
	case "reduce":
		return ReducePhase, nil
	}
}

type Task struct {
	instance interface{}
	value    reflect.Value
}

func NewTask(instance interface{}) *Task {
	value := reflect.ValueOf(instance)
	return &Task{
		instance: instance,
		value:    value,
	}
}

func (t *Task) Run(phase Phase, r io.Reader, w io.WriteCloser) (err error) {
	var input Reader
	pairs := NewPairReader(r)
	output := NewPairWriter(w)

	var m reflect.Value
	var ok bool
	switch phase {
	default:
		return fmt.Errorf("Invalid phase %d", phase)
	case MapPhase:
		input = pairs
		m, ok = t.mapper()
	case CombinePhase:
		input = NewGroupedReader(pairs)
		m, ok = t.combiner()
	case ReducePhase:
		input = NewGroupedReader(pairs)
		m, ok = t.reducer()
	}
	if !ok {
		return fmt.Errorf("No phase %d for %s", phase, t.instance)
	}
	err = t.run(m, input, output)
	return
}

func (t *Task) run(m reflect.Value, input Reader, output Writer) (err error) {
	collector := NewWriterCollector(output)
	colValue := reflect.ValueOf(collector)

	defer func() {
		if e := output.Close(); e != nil && err == nil {
			err = e
		}
	}()

	var k, v interface{}
	for {
		k, v, err = input.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			log.Printf("Read error: %s", err)
			return
		}
		m.Call([]reflect.Value{
			reflect.ValueOf(k),
			reflect.ValueOf(v),
			colValue,
		})
	}
}

func (t *Task) mapper() (reflect.Value, bool) {
	return t.methodByName("Map")
}

func (t *Task) combiner() (reflect.Value, bool) {
	return t.methodByName("Combine")
}

func (t *Task) reducer() (reflect.Value, bool) {
	return t.methodByName("Reduce")
}

func (t *Task) methodByName(name string) (v reflect.Value, ok bool) {
	v = t.value.MethodByName(name)
	ok = v.Kind() == reflect.Func
	return
}
