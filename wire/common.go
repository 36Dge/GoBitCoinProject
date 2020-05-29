package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"encoding/binary"
	"io"
	"time"
)

const (
	//maxvarintpayload is the maximum payload size for a varalbe lenth integar
	MaxVarIntPayload = 9

	//binaryFreelistmaxitems is the number of buffers to keep in the free list
	//to use for binary serialization and deserialization .
	binaryFreeListMaxItems = 1024
)

var (
	//litteendian is a converience variable since binary.littleendian is
	//quite long.
	littleEndian = binary.LittleEndian

	//bigendian is a convenience variable since binary.bigendian is quite
	//long
	bigEndian = binary.BigEndian
)

//binaryfreelist defines a concurrent safe free list of byte slices (up to the
//maximum number defined by the binaryfreelistmaxitems constant )that have a
//cap of 8(thus it supports up to uint64)it is used to provide temporary buffers
//for serializing and deserializing primitive numbers to and from their binary
//encoding in order to greatly reduce the number of allocations required.

//for convenience ,funcion are provided for each of primitive unsinged integers
//that automatically obtain a buffer from the free list, perform the
//necessary binary conversion, read from or write to the given io.Reader or
// io.Writer, and return the buffer to the free list.

type binaryFreeList chan []byte

//borrow returns a byte slice from the free list with a lenght of 8.a new buffer
//is allocated if there are not any available on the free list.
func (l binaryFreeList) Borrow() []byte {
	var buf []byte
	select {
	case buf = <-l:
	default:
		buf = make([]byte, 8)
	}
	return buf[:8]
}

//return puts the provided byte slice back on the free list .the buffer must
//have been obtained via the borrow fucction and therefore have a cap of 8
func (l binaryFreeList) Return(buf []byte) {
	select {
	case l <- buf:
	default:
		//let it go to the garbage collector
	}

}

//uint8 reads a single byte form the provided reader using a buffer form the
//free list and returns it as a uint8
func (l binaryFreeList) Uint8(r io.Reader) (uint8, error) {
	buf := l.Borrow()[:1]
	if _, err := io.ReadFull(r, buf); err != nil {
		l.Return(buf)
		return 0, err
	}
	rv := buf[0]
	l.Return(buf)
	return rv, nil
}

//uint16 reads two bytes from the provided reader using a buffer from the
//free list.converts it to a number using the provided type order .and return
//the resulting uint16
func (l binaryFreeList) Uint16(r io.Reader, byteOrder binary.ByteOrder) (uint16, error) {
	buf := l.Borrow()[:2]
	if _, err := io.ReadFull(r, buf); err != nil {
		l.Return(buf)
		return 0, err
	}
	rv := byteOrder.Uint16(buf)
	l.Return(buf)
	return rv, nil

}

//uint32 reads four bytes from the provided reader using a buffer from the the
//converts it to a number using the provided byte order .and returns the resulting
//uint32
func (l binaryFreeList) Uint32(r io.Reader, byteOrder binary.ByteOrder) (uint32, error) {
	buf := l.Borrow()[:4]
	if _, err := io.ReadFull(r, buf); err != nil {
		l.Return(buf)
		return 0, err
	}
	rv := byteOrder.Uint32(buf)
	l.Return(buf)
	return rv, nil

}

//uint64 reads eight bytes from the provided reader using a buffer form the
//free list ,converts it to a number using the provided byte order and return
//the resulting uint64
func (l binaryFreeList) Uint64(r io.Reader, byteOrder binary.ByteOrder) (uint64, error) {
	buf := l.Borrow()[:8]
	if _, err := io.ReadFull(r, buf); err != nil {
		l.Return(buf)
		return 0, err
	}
	rv := byteOrder.Uint64(buf)
	l.Return(buf)
	return rv, nil
}

// putuint8 copies provided uint8 into a buffer from free list and
// writes the resulting byte to the given writer
func (l binaryFreeList) PutUint8(w io.Writer, val uint8) error {
	buf := l.Borrow()[:1]
	buf[0] = val
	_, err := w.Write(buf)
	l.Return(buf)
	return err
}

// PutUint16 serializes the provided uint16 using the given byte order into a
// buffer from the free list and writes the resulting two bytes to the given
// writer.
func (l binaryFreeList) PutUint16(w io.Writer, byteOrder binary.ByteOrder, val uint16) error {
	buf := l.Borrow()[:2]
	byteOrder.PutUint16(buf, val)
	_, err := w.Write(buf)
	l.Return(buf)
	return err
}

// PutUint32 serializes the provided uint32 using the given byte order into a
// buffer from the free list and writes the resulting four bytes to the given
// writer.
func (l binaryFreeList) PutUint32(w io.Writer, byteOrder binary.ByteOrder, val uint32) error {
	buf := l.Borrow()[:4]
	byteOrder.PutUint32(buf, val)
	_, err := w.Write(buf)
	l.Return(buf)
	return err
}

// PutUint64 serializes the provided uint64 using the given byte order into a
// buffer from the free list and writes the resulting eight bytes to the given
// writer.
func (l binaryFreeList) PutUint64(w io.Writer, byteOrder binary.ByteOrder, val uint64) error {
	buf := l.Borrow()[:8]
	byteOrder.PutUint64(buf, val)
	_, err := w.Write(buf)
	l.Return(buf)
	return err
}

//binaryserializer provides a free list of buffers to use for serializing and
//deserializing primitive integer values to and from io.reader and io.wrirer
var binarySerializer binaryFreeList = make(chan []byte, binaryFreeListMaxItems)

//errnoncan.. is the common format string used for non-canonically encoded variable
//length integer errors
var errNonCanonicalVarInt = "non-canonical varint %x - discriminant %x must " +
	"encode a value greater than %x"

// uint32Time represents a unix timestamp encoded with a uint32.  It is used as
// a way to signal the readElement function how to decode a timestamp into a Go
// time.Time since it is otherwise ambiguous.
type uint32Time time.Time

// int64Time represents a unix timestamp encoded with a int64.  It is used as
// a way to signal the readElement function how to decode a timestamp into a Go
// time.Time since it is otherwise ambiguous.

type int64Time time.Time

//readelement reads the next sequence of bytes from r using little endian depending
//on the concrete type of element pointed to
func readElement(r io.Reader, element interface{}) error {

	//attmept to read the element based on the concrete type via fast type
	//assertions first
	switch e := element.(type) {
	case *int32:
		rv, err := binarySerializer.Uint32(r, littleEndian)
		if err != nil {
			return err
		}
		*e = int32(rv)
		return nil

	case *uint32:
		rv, err := binarySerializer.Uint32(r, littleEndian)
		if err != nil {
			return err
		}
		*e = rv
		return nil

	case *int64:
		rv, err := binarySerializer.Uint64(r, littleEndian)
		if err != nil {
			return err
		}
		*e = int64(rv)
		return nil

	case *bool:
		rv, err := binarySerializer.Uint8(r)
		if err != nil {
			return err
		}
		if rv == 0x00 {
			*e = false
		} else {
			*e = true
		}

		return nil

	//unix timestamp encoded as a uint32
	case *uint32Time:
		rv, err := binarySerializer.Uint32(r, binary.LittleEndian)
		if err != nil {
			return err
		}
		*e = uint32Time(time.Unix(int64(rv), 0))
		return nil

	//unix timestamp enconded as an int64
	case *int64Time:
		rv, err := binarySerializer.Uint64(r, binary.LittleEndian)
		if err != nil {
			return err
		}
		*e = int64Time(time.Unix(int64(rv), 0))
		return nil

	//message header checksum
	case *[4]byte:
		_, err := io.ReadFull(r, e[:])
		if err != nil {
			return err
		}
		return nil
		// message header command
	case *[CommandSize]uint8:
		_, err := io.ReadFull(r, e[:])
		if err != nil {
			return err
		}
		return nil

	//IP address
	case *[16]byte:
		_, err := io.ReadFull(r, e[:])
		if err != nil {
			return err
		}
		return nil

	case *ServiceFlag:
		rv, err := binarySerializer.Uint64(r, littleEndian)
		if err != nil {
			return err
		}
		*e = ServiceFlag(rv)
		return nil

	case *InvType:
		rv, err := binarySerializer.Uint32(r, littleEndian)
		if err != nil {
			return err
		}
		*e = InvType(rv)
		return nil

	case *BitcoinNet:
		rv, err := binarySerializer.Uint32(r, littleEndian)
		if err != nil {
			return err
		}
		*e = BitcoinNet(rv)
		return nil

	case *RejectCode:
		rv, err := binarySerializer.Uint8(r)
		if err != nil {
			return err
		}

		*e = RejectCode(rv)
		return nil

	case *BloomUpdateType:
		rv, err := binarySerializer.Uint8(r)
		if err != nil {
			return err
		}
		*e = BloomUpdateType(rv)
		return nil

	}
	//fall back to the slower binary.Read if a fast path was not available
	//above

	return binary.Read(r, littleEndian, element)

}

//readelements reads multiple items from r. it is equivalment to multiple
//calls to readElement.
func readElements(r io.Reader, elements ...interface{}) error {
	for _, element := range elements {
		err := readElement(r, element)
		if err != nil {
			return err
		}
	}
	return nil
}

//writeelement wirite the little endian representation of element to w
func writeElement(w io.Writer, element interface{}) error {
	// Attempt to write the element based on the concrete type via fast
	// type assertions first.
	switch e := element.(type) {
	case int32:
		err := binarySerializer.PutUint32(w, littleEndian, uint32(e))
		if err != nil {
			return err
		}
		return nil

	case uint32:
		err := binarySerializer.PutUint32(w, littleEndian, e)
		if err != nil {
			return err
		}
		return nil

	case int64:
		err := binarySerializer.PutUint64(w, littleEndian, uint64(e))
		if err != nil {
			return err
		}
		return nil

	case uint64:
		err := binarySerializer.PutUint64(w, littleEndian, e)
		if err != nil {
			return err
		}
		return nil

	case bool:
		var err error
		if e {
			err = binarySerializer.PutUint8(w, 0x01)
		} else {
			err = binarySerializer.PutUint8(w, 0x00)
		}
		if err != nil {
			return err
		}
		return nil

	// Message header checksum.
	case [4]byte:
		_, err := w.Write(e[:])
		if err != nil {
			return err
		}
		return nil

	// Message header command.
	case [CommandSize]uint8:
		_, err := w.Write(e[:])
		if err != nil {
			return err
		}
		return nil

	// IP address.
	case [16]byte:
		_, err := w.Write(e[:])
		if err != nil {
			return err
		}
		return nil

	case *chainhash.Hash:
		_, err := w.Write(e[:])
		if err != nil {
			return err
		}
		return nil

	case ServiceFlag:
		err := binarySerializer.PutUint64(w, littleEndian, uint64(e))
		if err != nil {
			return err
		}
		return nil

	case InvType:
		err := binarySerializer.PutUint32(w, littleEndian, uint32(e))
		if err != nil {
			return err
		}
		return nil

	case BitcoinNet:
		err := binarySerializer.PutUint32(w, littleEndian, uint32(e))
		if err != nil {
			return err
		}
		return nil

	case BloomUpdateType:
		err := binarySerializer.PutUint8(w, uint8(e))
		if err != nil {
			return err
		}
		return nil

	case RejectCode:
		err := binarySerializer.PutUint8(w, uint8(e))
		if err != nil {
			return err
		}
		return nil
	}

	// Fall back to the slower binary.Write if a fast path was not available
	// above.
	return binary.Write(w, littleEndian, element)


}

// writeElements writes multiple items to w.  It is equivalent to multiple
// calls to writeElement.
func writeElements(w io.Writer, elements ...interface{}) error {
	for _, element := range elements {
		err := writeElement(w, element)
		if err != nil {
			return err
		}
	}
	return nil
}


