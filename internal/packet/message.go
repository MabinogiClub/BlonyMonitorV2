package packet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"blonymonitorv2/internal/util"
)

var (
	be = binary.BigEndian
	le = binary.LittleEndian
)

type MessageElemType uint8

const (
	MessageElemTypeByte MessageElemType = 1 + iota
	MessageElemTypeShort
	MessageElemTypeInt
	MessageElemTypeLong
	MessageElemTypeFloat
	MessageElemTypeString
	MessageElemTypeBin
)

type Message []IMessageElem

func NewMessage(r io.Reader) (Message, error) {
	elemCount, _, err := util.ReadUvarint(r)
	if err != nil {
		return nil, err
	}

	l := make(Message, 0, elemCount)

	b := make([]byte, 1)
	// unused field
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	for i := range elemCount {
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, err
		}

		switch t := MessageElemType(b[0]); t {
		case MessageElemTypeByte:
			e, err := newMessageElemByte(r)
			if err != nil {
				return nil, err
			}

			l = append(l, e)

		case MessageElemTypeShort:
			e, err := newMessageElemShort(r)
			if err != nil {
				return nil, err
			}

			l = append(l, e)

		case MessageElemTypeInt:
			e, err := newMessageElemInt(r)
			if err != nil {
				return nil, err
			}

			l = append(l, e)

		case MessageElemTypeLong:
			e, err := newMessageElemLong(r)
			if err != nil {
				return nil, err
			}

			l = append(l, e)

		case MessageElemTypeFloat:
			e, err := newMessageElemFloat(r)
			if err != nil {
				return nil, err
			}

			l = append(l, e)

		case MessageElemTypeString:
			e, err := newMessageElemString(r)
			if err != nil {
				return nil, err
			}

			l = append(l, e)

		case MessageElemTypeBin:
			e, err := newMessageElemBin(r)
			if err != nil {
				return nil, err
			}

			l = append(l, e)

		default:
			return nil, fmt.Errorf("newGamePacket: unknown elem type %d %d", t, i)
		}
	}

	return l, nil
}

func (t *Message) Write(w io.Writer) error {
	if _, err := w.Write(t.Bytes()); err != nil {
		return err
	}

	return nil
}

func (t *Message) Bytes() []byte {
	buf := bytes.NewBuffer(nil)

	count := uint64(len(*t))
	countB := binary.AppendUvarint(nil, count)
	buf.Write(countB)
	buf.WriteByte(0)

	for _, v := range *t {
		buf.Write(v.Bytes())
	}

	return buf.Bytes()
}

func (t *Message) Len() uint64 {
	r := uint64(0)

	// body的长度
	// count := uint64(len(*t))
	// countB := binary.AppendUvarint(nil, count)

	// r += uint64(len(countB))
	// r += 1

	for _, v := range *t {
		r += v.Len()
	}

	return r
}

func (t *Message) DebugPrint() {
	for i, v := range *t {
		logger.Println("Message", i, v.Type(), v.String())
	}
}

type IMessageElem interface {
	Type() MessageElemType
	Data() interface{}
	Bytes() []byte
	Len() uint64
	String() string
}

var _ IMessageElem = (*MessageElemByte)(nil)

type MessageElemByte struct {
	value uint8
}

func (t *MessageElemByte) Type() MessageElemType {
	return MessageElemTypeByte
}

func (t *MessageElemByte) Data() interface{} {
	return t.value
}

func (t *MessageElemByte) Bytes() []byte {
	b := make([]byte, 2)
	b[0] = byte(MessageElemTypeByte)
	b[1] = t.value

	return b
}

func (t *MessageElemByte) Len() uint64 {
	return 2
}

func (t *MessageElemByte) String() string {
	return fmt.Sprintf("%v", t.value)
}

func newMessageElemByte(r io.Reader) (IMessageElem, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	return &MessageElemByte{
		value: b[0],
	}, nil
}

func NewMessageElemByte(v uint8) IMessageElem {
	return &MessageElemByte{
		value: v,
	}
}

type MessageElemShort struct {
	value uint16
}

func (t *MessageElemShort) Type() MessageElemType {
	return MessageElemTypeShort
}

func (t *MessageElemShort) Data() interface{} {
	return t.value
}

func (t *MessageElemShort) Bytes() []byte {
	b := make([]byte, 3)
	b[0] = byte(MessageElemTypeShort)
	be.PutUint16(b[1:], t.value)

	return b
}

func (t *MessageElemShort) Len() uint64 {
	return 3
}

func (t *MessageElemShort) String() string {
	return fmt.Sprintf("%v", t.value)
}

func newMessageElemShort(r io.Reader) (IMessageElem, error) {
	b := make([]byte, 2)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	return &MessageElemShort{
		value: be.Uint16(b),
	}, nil
}

func NewMessageElemShort(v uint16) IMessageElem {
	return &MessageElemShort{
		value: v,
	}
}

type MessageElemInt struct {
	value uint32
}

func (t *MessageElemInt) Type() MessageElemType {
	return MessageElemTypeInt
}

func (t *MessageElemInt) Data() interface{} {
	return t.value
}

func (t *MessageElemInt) Bytes() []byte {
	b := make([]byte, 5)
	b[0] = byte(MessageElemTypeInt)
	be.PutUint32(b[1:], t.value)

	return b
}

func (t *MessageElemInt) Len() uint64 {
	return 5
}

func (t *MessageElemInt) String() string {
	return fmt.Sprintf("%v", t.value)
}

func newMessageElemInt(r io.Reader) (IMessageElem, error) {
	b := make([]byte, 4)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	return &MessageElemInt{
		value: be.Uint32(b),
	}, nil
}

func NewMessageElemInt(v uint32) IMessageElem {
	return &MessageElemInt{
		value: v,
	}
}

type MessageElemLong struct {
	value uint64
}

func (t *MessageElemLong) Type() MessageElemType {
	return MessageElemTypeLong
}

func (t *MessageElemLong) Data() interface{} {
	return t.value
}

func (t *MessageElemLong) Bytes() []byte {
	b := make([]byte, 9)
	b[0] = byte(MessageElemTypeLong)
	be.PutUint64(b[1:], t.value)

	return b
}

func (t *MessageElemLong) Len() uint64 {
	return 9
}

func (t *MessageElemLong) String() string {
	return fmt.Sprintf("%v", t.value)
}

func newMessageElemLong(r io.Reader) (IMessageElem, error) {
	b := make([]byte, 8)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	return &MessageElemLong{
		value: be.Uint64(b),
	}, nil
}

func NewMessageElemLong(v uint64) IMessageElem {
	return &MessageElemLong{
		value: v,
	}
}

type MessageElemFloat struct {
	value float32
}

func (t *MessageElemFloat) Type() MessageElemType {
	return MessageElemTypeFloat
}

func (t *MessageElemFloat) Data() interface{} {
	return t.value
}

func (t *MessageElemFloat) Bytes() []byte {
	b := make([]byte, 5)
	b[0] = byte(MessageElemTypeFloat)
	le.PutUint32(b[1:], math.Float32bits(t.value))

	return b
}

func (t *MessageElemFloat) Len() uint64 {
	return 5
}

func (t *MessageElemFloat) String() string {
	return fmt.Sprintf("%v", t.value)
}

func newMessageElemFloat(r io.Reader) (IMessageElem, error) {
	b := make([]byte, 4)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	return &MessageElemFloat{
		value: math.Float32frombits(le.Uint32(b)),
	}, nil
}

func NewMessageElemFloat(v float32) IMessageElem {
	return &MessageElemFloat{
		value: v,
	}
}

type MessageElemString struct {
	value string
}

func (t *MessageElemString) Type() MessageElemType {
	return MessageElemTypeString
}

func (t *MessageElemString) Data() interface{} {
	return t.value
}

func (t *MessageElemString) Bytes() []byte {
	// type 1b, length 2b, value nb, null termination 1b
	b := make([]byte, 1+2+len(t.value)+1)
	b[0] = byte(MessageElemTypeString)
	be.PutUint16(b[1:], uint16(len(t.value)+1))
	copy(b[3:], t.value)

	return b
}

func (t *MessageElemString) Len() uint64 {
	return uint64(1 + 2 + len(t.value) + 1)
}

func (t *MessageElemString) String() string {
	return t.value
}

func newMessageElemString(r io.Reader) (IMessageElem, error) {
	b := make([]byte, 2)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	length := be.Uint16(b)

	if length == 0 {
		// ?
		return &MessageElemString{
			value: "",
		}, nil
	}

	if length > 2 {
		b = make([]byte, length)
	}

	if _, err := io.ReadFull(r, b[:length]); err != nil {
		return nil, err
	}

	return &MessageElemString{
		// null termination
		value: string(b[:length-1]),
	}, nil
}

func NewMessageElemString(v string) IMessageElem {
	return &MessageElemString{
		value: v,
	}
}

type MessageElemBin struct {
	value []byte
}

func (t *MessageElemBin) Type() MessageElemType {
	return MessageElemTypeBin
}

func (t *MessageElemBin) Data() interface{} {
	return t.value
}

func (t *MessageElemBin) Bytes() []byte {
	// type 1b, length 2b, value nb
	b := make([]byte, 1+2+len(t.value))
	b[0] = byte(MessageElemTypeBin)
	be.PutUint16(b[1:], uint16(len(t.value)))
	copy(b[3:], t.value)

	return b
}

func (t *MessageElemBin) Len() uint64 {
	return uint64(1 + 2 + len(t.value))
}

func (t *MessageElemBin) String() string {
	return fmt.Sprintf("%x", t.value)
}

func newMessageElemBin(r io.Reader) (IMessageElem, error) {
	b := make([]byte, 2)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	length := be.Uint16(b)

	if length == 0 {
		// ?
		return &MessageElemBin{
			value: nil,
		}, nil
	}

	if length > 2 {
		b = make([]byte, length)
	}

	if _, err := io.ReadFull(r, b[:length]); err != nil {
		return nil, err
	}

	return &MessageElemBin{
		value: b,
	}, nil
}

func NewMessageElemBin(v []byte) IMessageElem {
	return &MessageElemBin{
		value: v,
	}
}
