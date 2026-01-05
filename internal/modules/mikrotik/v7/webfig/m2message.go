// M2Message implements the RouterOS M2 message format used in WebFig protocol
package webfig

import (
	"encoding/binary"
)

// M2Message represents a RouterOS M2 message
// This is used for communication in the WebFig protocol
type M2Message struct {
	Bools    map[uint32]bool
	U32s     map[uint32]uint32
	Strings  map[uint32][]byte
	Raw      map[uint32][]byte
	ArrayU32 map[uint32][]uint32
}

// Message type constants
const (
	typeBoolean     uint32 = 0
	typeShortLength uint32 = 0x01000000
	typeUint32      uint32 = 0x08000000
	typeString      uint32 = 0x20000000
	typeRaw         uint32 = 0x30000000
	typeUint32Array uint32 = 0x88000000
)

// Standard RouterOS message variables
const (
	VarSysTo         uint32 = 0x00ff0001
	VarFrom          uint32 = 0x00ff0002
	VarReplyExpected uint32 = 0x00ff0005
	VarRequestID     uint32 = 0x00ff0006
	VarCommand       uint32 = 0x00ff0007
	VarErrorCode     uint32 = 0x00ff0008
	VarErrorString   uint32 = 0x00ff0009
	VarSessionID     uint32 = 0x00fe0001
)

// NewM2Message creates a new M2 message
func NewM2Message() *M2Message {
	return &M2Message{
		Bools:    make(map[uint32]bool),
		U32s:     make(map[uint32]uint32),
		Strings:  make(map[uint32][]byte),
		Raw:      make(map[uint32][]byte),
		ArrayU32: make(map[uint32][]uint32),
	}
}

// AddBool adds a boolean value to the message
func (msg *M2Message) AddBool(varname uint32, data bool) {
	msg.Bools[varname] = data
}

// AddU32 adds a uint32 value to the message
func (msg *M2Message) AddU32(varname uint32, data uint32) {
	msg.U32s[varname] = data
}

// AddString adds a string value to the message
func (msg *M2Message) AddString(varname uint32, data []byte) {
	msg.Strings[varname] = make([]byte, len(data))
	copy(msg.Strings[varname], data)
}

// AddU32Array adds a uint32 array to the message
func (msg *M2Message) AddU32Array(varname uint32, data []uint32) {
	msg.ArrayU32[varname] = append(msg.ArrayU32[varname], data...)
}

// Serialize converts the M2 message to binary format
func (msg *M2Message) Serialize() []byte {
	serialized := []byte{}

	// Serialize booleans
	for varname, value := range msg.Bools {
		binaryBool := make([]byte, 4)
		if value {
			varname |= typeShortLength
		}
		binary.LittleEndian.PutUint32(binaryBool, varname)
		serialized = append(serialized, binaryBool...)
	}

	// Serialize uint32 values
	for varname, value := range msg.U32s {
		varname |= typeUint32
		binaryUint32 := make([]byte, 4)

		if value > 255 {
			binary.LittleEndian.PutUint32(binaryUint32, varname)
			serialized = append(serialized, binaryUint32...)
			binary.LittleEndian.PutUint32(binaryUint32, value)
			serialized = append(serialized, binaryUint32...)
		} else {
			varname |= typeShortLength
			binary.LittleEndian.PutUint32(binaryUint32, varname)
			serialized = append(serialized, binaryUint32...)
			serialized = append(serialized, byte(value&0x000000ff))
		}
	}

	// Serialize strings
	for varname, value := range msg.Strings {
		varname |= typeString
		binaryString := make([]byte, 4)

		if len(value) > 255 {
			binary.LittleEndian.PutUint32(binaryString, varname)
			serialized = append(serialized, binaryString...)
			binaryLength := make([]byte, 2)
			binary.LittleEndian.PutUint16(binaryLength, uint16(len(value)))
			serialized = append(serialized, binaryLength...)
		} else {
			varname |= typeShortLength
			binary.LittleEndian.PutUint32(binaryString, varname)
			serialized = append(serialized, binaryString...)
			serialized = append(serialized, byte(len(value)))
		}
		serialized = append(serialized, value...)
	}

	// Serialize raw data
	for varname, value := range msg.Raw {
		varname |= typeRaw
		binaryString := make([]byte, 4)

		if len(value) > 255 {
			binary.LittleEndian.PutUint32(binaryString, varname)
			serialized = append(serialized, binaryString...)
			binaryLength := make([]byte, 2)
			binary.LittleEndian.PutUint16(binaryLength, uint16(len(value)))
			serialized = append(serialized, binaryLength...)
		} else {
			varname |= typeShortLength
			binary.LittleEndian.PutUint32(binaryString, varname)
			serialized = append(serialized, binaryString...)
			serialized = append(serialized, byte(len(value)))
		}
		serialized = append(serialized, value...)
	}

	// Serialize uint32 arrays
	for varname, value := range msg.ArrayU32 {
		varname |= typeUint32Array
		binaryArray := make([]byte, 4)
		binary.LittleEndian.PutUint32(binaryArray, varname)
		serialized = append(serialized, binaryArray...)

		binaryLength := make([]byte, 2)
		binary.LittleEndian.PutUint16(binaryLength, uint16(len(value)))
		serialized = append(serialized, binaryLength...)

		for _, entry := range value {
			binaryEntry := make([]byte, 4)
			binary.LittleEndian.PutUint32(binaryEntry, entry)
			serialized = append(serialized, binaryEntry...)
		}
	}

	return serialized
}

// ParseM2Message parses a binary M2 message into a structured format
func ParseM2Message(data []byte, msg *M2Message) bool {
	if len(data) < 4 {
		return false
	}

	// Check if message starts with M2 header
	if len(data) >= 4 && data[2] == 'M' && data[3] == '2' {
		// Skip M2 header
		data = data[4:]
	}

	for len(data) > 4 {
		varTypeName := binary.LittleEndian.Uint32(data)
		varType := varTypeName & 0xf8000000
		varName := varTypeName & 0x00ffffff
		data = data[4:]

		switch varType {
		case typeBoolean:
			msg.Bools[varName] = (varTypeName & typeShortLength) != 0
		case typeUint32:
			if (varTypeName & typeShortLength) != 0 {
				if len(data) == 0 {
					return false
				}
				msg.U32s[varName] = uint32(data[0])
				data = data[1:]
			} else {
				if len(data) < 4 {
					return false
				}
				msg.U32s[varName] = binary.LittleEndian.Uint32(data)
				data = data[4:]
			}
		case typeString:
			if !handleStringOrRaw(varTypeName, varName, &data, &msg.Strings) {
				return false
			}
		case typeRaw:
			if !handleStringOrRaw(varTypeName, varName, &data, &msg.Raw) {
				return false
			}
		case typeUint32Array:
			if len(data) <= 2 {
				return false
			}
			arrayEntries := int(binary.LittleEndian.Uint16(data))
			data = data[2:]
			if len(data) < (arrayEntries * 4) {
				return false
			}

			for i := 0; i < arrayEntries; i++ {
				msg.ArrayU32[varName] = append(msg.ArrayU32[varName], binary.LittleEndian.Uint32(data))
				data = data[4:]
			}
		default:
			// Unknown type, skip it
			return false
		}
	}

	return true
}

// handleStringOrRaw handles string and raw data types
func handleStringOrRaw(varTypeName uint32, varName uint32, data *[]byte, storage *map[uint32][]byte) bool {
	if len(*data) <= 2 {
		return false
	}

	length := int(binary.LittleEndian.Uint16(*data))
	if (varTypeName & typeShortLength) != 0 {
		length = int((*data)[0])
		*data = (*data)[1:]
	} else {
		*data = (*data)[2:]
	}

	if len(*data) < length {
		return false
	}

	(*storage)[varName] = make([]byte, length)
	copy((*storage)[varName], *data)
	*data = (*data)[length:]

	return true
}
