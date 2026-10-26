package k9o_keep_core_ffi_uniffi

// #include <k9o_keep_core_ffi_uniffi.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// This is needed, because as of go 1.24
// type RustBuffer C.RustBuffer cannot have methods,
// RustBuffer is treated as non-local type
type GoRustBuffer struct {
	inner C.RustBuffer
}

type RustBufferI interface {
	AsReader() *bytes.Reader
	Free()
	ToGoBytes() []byte
	Data() unsafe.Pointer
	Len() uint64
	Capacity() uint64
}

func RustBufferFromExternal(b RustBufferI) GoRustBuffer {
	return GoRustBuffer{
		inner: C.RustBuffer{
			capacity: C.uint64_t(b.Capacity()),
			len:      C.uint64_t(b.Len()),
			data:     (*C.uchar)(b.Data()),
		},
	}
}

func (cb GoRustBuffer) Capacity() uint64 {
	return uint64(cb.inner.capacity)
}

func (cb GoRustBuffer) Len() uint64 {
	return uint64(cb.inner.len)
}

func (cb GoRustBuffer) Data() unsafe.Pointer {
	return unsafe.Pointer(cb.inner.data)
}

func (cb GoRustBuffer) AsReader() *bytes.Reader {
	b := unsafe.Slice((*byte)(cb.inner.data), C.uint64_t(cb.inner.len))
	return bytes.NewReader(b)
}

func (cb GoRustBuffer) Free() {
	rustCall(func(status *C.RustCallStatus) bool {
		C.ffi_k9o_keep_core_ffi_uniffi_rustbuffer_free(cb.inner, status)
		return false
	})
}

func (cb GoRustBuffer) ToGoBytes() []byte {
	return C.GoBytes(unsafe.Pointer(cb.inner.data), C.int(cb.inner.len))
}

func stringToRustBuffer(str string) C.RustBuffer {
	return bytesToRustBuffer([]byte(str))
}

func bytesToRustBuffer(b []byte) C.RustBuffer {
	if len(b) == 0 {
		return C.RustBuffer{}
	}
	// We can pass the pointer along here, as it is pinned
	// for the duration of this call
	foreign := C.ForeignBytes{
		len:  C.int(len(b)),
		data: (*C.uchar)(unsafe.Pointer(&b[0])),
	}

	return rustCall(func(status *C.RustCallStatus) C.RustBuffer {
		return C.ffi_k9o_keep_core_ffi_uniffi_rustbuffer_from_bytes(foreign, status)
	})
}

type BufLifter[GoType any] interface {
	Lift(value RustBufferI) GoType
}

type BufLowerer[GoType any] interface {
	Lower(value GoType) C.RustBuffer
}

type BufReader[GoType any] interface {
	Read(reader io.Reader) GoType
}

type BufWriter[GoType any] interface {
	Write(writer io.Writer, value GoType)
}

func LowerIntoRustBuffer[GoType any](bufWriter BufWriter[GoType], value GoType) C.RustBuffer {
	// This might be not the most efficient way but it does not require knowing allocation size
	// beforehand
	var buffer bytes.Buffer
	bufWriter.Write(&buffer, value)

	bytes, err := io.ReadAll(&buffer)
	if err != nil {
		panic(fmt.Errorf("reading written data: %w", err))
	}
	return bytesToRustBuffer(bytes)
}

func LiftFromRustBuffer[GoType any](bufReader BufReader[GoType], rbuf RustBufferI) GoType {
	defer rbuf.Free()
	reader := rbuf.AsReader()
	item := bufReader.Read(reader)
	if reader.Len() > 0 {
		// TODO: Remove this
		leftover, _ := io.ReadAll(reader)
		panic(fmt.Errorf("Junk remaining in buffer after lifting: %s", string(leftover)))
	}
	return item
}

func rustCallWithError[E any, U any](converter BufReader[*E], callback func(*C.RustCallStatus) U) (U, *E) {
	var status C.RustCallStatus
	returnValue := callback(&status)
	err := checkCallStatus(converter, status)
	return returnValue, err
}

func checkCallStatus[E any](converter BufReader[*E], status C.RustCallStatus) *E {
	switch status.code {
	case 0:
		return nil
	case 1:
		return LiftFromRustBuffer(converter, GoRustBuffer{inner: status.errorBuf})
	case 2:
		// when the rust code sees a panic, it tries to construct a rustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{inner: status.errorBuf})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		panic(fmt.Errorf("unknown status code: %d", status.code))
	}
}

func checkCallStatusUnknown(status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		panic(fmt.Errorf("function not returning an error returned an error"))
	case 2:
		// when the rust code sees a panic, it tries to construct a C.RustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: status.errorBuf,
			})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func rustCall[U any](callback func(*C.RustCallStatus) U) U {
	returnValue, err := rustCallWithError[error](nil, callback)
	if err != nil {
		panic(err)
	}
	return returnValue
}

type NativeError interface {
	AsError() error
}

func writeInt8(writer io.Writer, value int8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint8(writer io.Writer, value uint8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt16(writer io.Writer, value int16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint16(writer io.Writer, value uint16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt32(writer io.Writer, value int32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint32(writer io.Writer, value uint32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt64(writer io.Writer, value int64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint64(writer io.Writer, value uint64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat32(writer io.Writer, value float32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat64(writer io.Writer, value float64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func readInt8(reader io.Reader) int8 {
	var result int8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint8(reader io.Reader) uint8 {
	var result uint8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt16(reader io.Reader) int16 {
	var result int16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint16(reader io.Reader) uint16 {
	var result uint16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt32(reader io.Reader) int32 {
	var result int32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint32(reader io.Reader) uint32 {
	var result uint32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt64(reader io.Reader) int64 {
	var result int64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint64(reader io.Reader) uint64 {
	var result uint64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat32(reader io.Reader) float32 {
	var result float32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat64(reader io.Reader) float64 {
	var result float64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func init() {

	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 26
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_k9o_keep_core_ffi_uniffi_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("k9o_keep_core_ffi_uniffi: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_func_uses_test_trust_root()
		})
		if checksum != 4319 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_func_uses_test_trust_root: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_active_ruleset_is_signed()
		})
		if checksum != 62300 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_active_ruleset_is_signed: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_classify_local()
		})
		if checksum != 33908 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_classify_local: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_core_version()
		})
		if checksum != 48803 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_core_version: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_deprecated_kids()
		})
		if checksum != 35040 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_deprecated_kids: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_inspect_input()
		})
		if checksum != 63743 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_inspect_input: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_key_manifest_sequence()
		})
		if checksum != 29223 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_key_manifest_sequence: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_load_key_manifest()
		})
		if checksum != 25257 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_load_key_manifest: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_load_ruleset_signed()
		})
		if checksum != 51434 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_load_ruleset_signed: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_load_ruleset_unsigned()
		})
		if checksum != 35882 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_load_ruleset_unsigned: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_output_filter()
		})
		if checksum != 9172 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_output_filter: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_ruleset_digest()
		})
		if checksum != 50254 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_ruleset_digest: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_ruleset_version()
		})
		if checksum != 51524 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_ruleset_version: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_signed_floor()
		})
		if checksum != 22085 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_signed_floor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_trusted_kids()
		})
		if checksum != 64294 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_trusted_kids: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_verify_manifest()
		})
		if checksum != 29555 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepcore_verify_manifest: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepstreamfilter_finish()
		})
		if checksum != 43598 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepstreamfilter_finish: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepstreamfilter_push()
		})
		if checksum != 21368 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepstreamfilter_push: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepstreamfilter_report()
		})
		if checksum != 23662 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_method_keepstreamfilter_report: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_k9o_keep_core_ffi_uniffi_checksum_constructor_keepcore_new()
		})
		if checksum != 57881 {
			// If this happens try cleaning and rebuilding your project
			panic("k9o_keep_core_ffi_uniffi: uniffi_k9o_keep_core_ffi_uniffi_checksum_constructor_keepcore_new: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint8 struct{}

var FfiConverterUint8INSTANCE = FfiConverterUint8{}

func (FfiConverterUint8) Lower(value uint8) C.uint8_t {
	return C.uint8_t(value)
}

func (FfiConverterUint8) Write(writer io.Writer, value uint8) {
	writeUint8(writer, value)
}

func (FfiConverterUint8) Lift(value C.uint8_t) uint8 {
	return uint8(value)
}

func (FfiConverterUint8) Read(reader io.Reader) uint8 {
	return readUint8(reader)
}

type FfiDestroyerUint8 struct{}

func (FfiDestroyerUint8) Destroy(_ uint8) {}

type FfiConverterUint32 struct{}

var FfiConverterUint32INSTANCE = FfiConverterUint32{}

func (FfiConverterUint32) Lower(value uint32) C.uint32_t {
	return C.uint32_t(value)
}

func (FfiConverterUint32) Write(writer io.Writer, value uint32) {
	writeUint32(writer, value)
}

func (FfiConverterUint32) Lift(value C.uint32_t) uint32 {
	return uint32(value)
}

func (FfiConverterUint32) Read(reader io.Reader) uint32 {
	return readUint32(reader)
}

type FfiDestroyerUint32 struct{}

func (FfiDestroyerUint32) Destroy(_ uint32) {}

type FfiConverterUint64 struct{}

var FfiConverterUint64INSTANCE = FfiConverterUint64{}

func (FfiConverterUint64) Lower(value uint64) C.uint64_t {
	return C.uint64_t(value)
}

func (FfiConverterUint64) Write(writer io.Writer, value uint64) {
	writeUint64(writer, value)
}

func (FfiConverterUint64) Lift(value C.uint64_t) uint64 {
	return uint64(value)
}

func (FfiConverterUint64) Read(reader io.Reader) uint64 {
	return readUint64(reader)
}

type FfiDestroyerUint64 struct{}

func (FfiDestroyerUint64) Destroy(_ uint64) {}

type FfiConverterInt64 struct{}

var FfiConverterInt64INSTANCE = FfiConverterInt64{}

func (FfiConverterInt64) Lower(value int64) C.int64_t {
	return C.int64_t(value)
}

func (FfiConverterInt64) Write(writer io.Writer, value int64) {
	writeInt64(writer, value)
}

func (FfiConverterInt64) Lift(value C.int64_t) int64 {
	return int64(value)
}

func (FfiConverterInt64) Read(reader io.Reader) int64 {
	return readInt64(reader)
}

type FfiDestroyerInt64 struct{}

func (FfiDestroyerInt64) Destroy(_ int64) {}

type FfiConverterBool struct{}

var FfiConverterBoolINSTANCE = FfiConverterBool{}

func (FfiConverterBool) Lower(value bool) C.int8_t {
	if value {
		return C.int8_t(1)
	}
	return C.int8_t(0)
}

func (FfiConverterBool) Write(writer io.Writer, value bool) {
	if value {
		writeInt8(writer, 1)
	} else {
		writeInt8(writer, 0)
	}
}

func (FfiConverterBool) Lift(value C.int8_t) bool {
	return value != 0
}

func (FfiConverterBool) Read(reader io.Reader) bool {
	return readInt8(reader) != 0
}

type FfiDestroyerBool struct{}

func (FfiDestroyerBool) Destroy(_ bool) {}

type FfiConverterString struct{}

var FfiConverterStringINSTANCE = FfiConverterString{}

func (FfiConverterString) Lift(rb RustBufferI) string {
	defer rb.Free()
	reader := rb.AsReader()
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("reading reader: %w", err))
	}
	return string(b)
}

func (FfiConverterString) Read(reader io.Reader) string {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading string, expected %d, read %d", length, read_length))
	}
	return string(buffer)
}

func (FfiConverterString) Lower(value string) C.RustBuffer {
	return stringToRustBuffer(value)
}

func (FfiConverterString) Write(writer io.Writer, value string) {
	if len(value) > math.MaxInt32 {
		panic("String is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := io.WriteString(writer, value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing string, expected %d, written %d", len(value), write_length))
	}
}

type FfiDestroyerString struct{}

func (FfiDestroyerString) Destroy(_ string) {}

type FfiConverterBytes struct{}

var FfiConverterBytesINSTANCE = FfiConverterBytes{}

func (c FfiConverterBytes) Lower(value []byte) C.RustBuffer {
	return LowerIntoRustBuffer[[]byte](c, value)
}

func (c FfiConverterBytes) Write(writer io.Writer, value []byte) {
	if len(value) > math.MaxInt32 {
		panic("[]byte is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := writer.Write(value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing []byte, expected %d, written %d", len(value), write_length))
	}
}

func (c FfiConverterBytes) Lift(rb RustBufferI) []byte {
	return LiftFromRustBuffer[[]byte](c, rb)
}

func (c FfiConverterBytes) Read(reader io.Reader) []byte {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading []byte, expected %d, read %d", length, read_length))
	}
	return buffer
}

type FfiDestroyerBytes struct{}

func (FfiDestroyerBytes) Destroy(_ []byte) {}

// Below is an implementation of synchronization requirements outlined in the link.
// https://github.com/mozilla/uniffi-rs/blob/0dc031132d9493ca812c3af6e7dd60ad2ea95bf0/uniffi_bindgen/src/bindings/kotlin/templates/ObjectRuntime.kt#L31

type FfiObject struct {
	pointer       unsafe.Pointer
	callCounter   atomic.Int64
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer
	freeFunction  func(unsafe.Pointer, *C.RustCallStatus)
	destroyed     atomic.Bool
}

func newFfiObject(
	pointer unsafe.Pointer,
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer,
	freeFunction func(unsafe.Pointer, *C.RustCallStatus),
) FfiObject {
	return FfiObject{
		pointer:       pointer,
		cloneFunction: cloneFunction,
		freeFunction:  freeFunction,
	}
}

func (ffiObject *FfiObject) incrementPointer(debugName string) unsafe.Pointer {
	for {
		counter := ffiObject.callCounter.Load()
		if counter <= -1 {
			panic(fmt.Errorf("%v object has already been destroyed", debugName))
		}
		if counter == math.MaxInt64 {
			panic(fmt.Errorf("%v object call counter would overflow", debugName))
		}
		if ffiObject.callCounter.CompareAndSwap(counter, counter+1) {
			break
		}
	}

	return rustCall(func(status *C.RustCallStatus) unsafe.Pointer {
		return ffiObject.cloneFunction(ffiObject.pointer, status)
	})
}

func (ffiObject *FfiObject) decrementPointer() {
	if ffiObject.callCounter.Add(-1) == -1 {
		ffiObject.freeRustArcPtr()
	}
}

func (ffiObject *FfiObject) destroy() {
	if ffiObject.destroyed.CompareAndSwap(false, true) {
		if ffiObject.callCounter.Add(-1) == -1 {
			ffiObject.freeRustArcPtr()
		}
	}
}

func (ffiObject *FfiObject) freeRustArcPtr() {
	rustCall(func(status *C.RustCallStatus) int32 {
		ffiObject.freeFunction(ffiObject.pointer, status)
		return 0
	})
}

type KeepCoreInterface interface {
	// Whether the ruleset in force came from a verified load. False when none is loaded or an unsigned load
	// replaced a verified one.
	ActiveRulesetIsSigned() bool
	// Local L3 stand-in classification (no receipt).
	ClassifyLocal(text string) string
	CoreVersion() string
	// The trusted `kid`s the key manifest flags `deprecated`, sorted.
	DeprecatedKids() []string
	// Inspect an input (normalize -> deny -> local verdict). `declared_class`
	// is the manifest's waypoint_class meter hint — it never gates the decision.
	InspectInput(text string, declaredClass *string) KeepVerdict
	// The `sequence` of the key manifest in force (0 before one is loaded).
	KeyManifestSequence() uint64
	// Verify a Barbican key manifest (compact JWS) against the root compiled into the core and, only if it
	// verifies, REPLACE the trusted ruleset-signing keys. `now_secs` is epoch seconds. Returns the `sequence`.
	LoadKeyManifest(jws string, nowSecs uint64) (uint64, error)
	// Load a signed ruleset. The core verifies it under a key from the verified key manifest; on any refusal the
	// previous ruleset stays in force and the error `reason` is a closed token.
	LoadRulesetSigned(blob KeepSignedRulesetBlob) (uint64, error)
	// Static-bundle ruleset load (the SDK's embedded rules; provenance vouched
	// for by the package). Anti-rollback still enforced by the core. Returns
	// the accepted version.
	LoadRulesetUnsigned(blob KeepRulesetBlob) (uint64, error)
	// Open an output stream-filter bound to the active ruleset's exfil markers.
	OutputFilter() *KeepStreamFilter
	// Lowercase hex SHA-256 of a ruleset's canonical signing bytes (what a manifest's `artifactHash` names).
	// A digest only: it says nothing about trust, and the canonical bytes stay in the core.
	RulesetDigest(blob KeepSignedRulesetBlob) (string, error)
	RulesetVersion() uint64
	// The highest VERIFIED ruleset version loaded (0 = none): the anti-rollback floor for signed updates. A refresh
	// client compares a published version against THIS, not the version in force, which may be an unsigned load.
	SignedFloor() uint64
	// The signing `kid`s currently trusted, sorted.
	TrustedKids() []string
	// Verify a ruleset manifest (compact JWS) under a trusted key and return its authenticated fields.
	VerifyManifest(jws string) (KeepManifestInfo, error)
}
type KeepCore struct {
	ffiObject FfiObject
}

func NewKeepCore(config KeepConfig) *KeepCore {
	return FfiConverterKeepCoreINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_k9o_keep_core_ffi_uniffi_fn_constructor_keepcore_new(FfiConverterKeepConfigINSTANCE.Lower(config), _uniffiStatus)
	}))
}

// Whether the ruleset in force came from a verified load. False when none is loaded or an unsigned load
// replaced a verified one.
func (_self *KeepCore) ActiveRulesetIsSigned() bool {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_active_ruleset_is_signed(
			_pointer, _uniffiStatus)
	}))
}

// Local L3 stand-in classification (no receipt).
func (_self *KeepCore) ClassifyLocal(text string) string {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_classify_local(
				_pointer, FfiConverterStringINSTANCE.Lower(text), _uniffiStatus),
		}
	}))
}

func (_self *KeepCore) CoreVersion() string {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_core_version(
				_pointer, _uniffiStatus),
		}
	}))
}

// The trusted `kid`s the key manifest flags `deprecated`, sorted.
func (_self *KeepCore) DeprecatedKids() []string {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_deprecated_kids(
				_pointer, _uniffiStatus),
		}
	}))
}

// Inspect an input (normalize -> deny -> local verdict). `declared_class`
// is the manifest's waypoint_class meter hint — it never gates the decision.
func (_self *KeepCore) InspectInput(text string, declaredClass *string) KeepVerdict {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterKeepVerdictINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_inspect_input(
				_pointer, FfiConverterStringINSTANCE.Lower(text), FfiConverterOptionalStringINSTANCE.Lower(declaredClass), _uniffiStatus),
		}
	}))
}

// The `sequence` of the key manifest in force (0 before one is loaded).
func (_self *KeepCore) KeyManifestSequence() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_key_manifest_sequence(
			_pointer, _uniffiStatus)
	}))
}

// Verify a Barbican key manifest (compact JWS) against the root compiled into the core and, only if it
// verifies, REPLACE the trusted ruleset-signing keys. `now_secs` is epoch seconds. Returns the `sequence`.
func (_self *KeepCore) LoadKeyManifest(jws string, nowSecs uint64) (uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[KeepError](FfiConverterKeepError{}, func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_load_key_manifest(
			_pointer, FfiConverterStringINSTANCE.Lower(jws), FfiConverterUint64INSTANCE.Lower(nowSecs), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint64INSTANCE.Lift(_uniffiRV), nil
	}
}

// Load a signed ruleset. The core verifies it under a key from the verified key manifest; on any refusal the
// previous ruleset stays in force and the error `reason` is a closed token.
func (_self *KeepCore) LoadRulesetSigned(blob KeepSignedRulesetBlob) (uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[KeepError](FfiConverterKeepError{}, func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_load_ruleset_signed(
			_pointer, FfiConverterKeepSignedRulesetBlobINSTANCE.Lower(blob), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint64INSTANCE.Lift(_uniffiRV), nil
	}
}

// Static-bundle ruleset load (the SDK's embedded rules; provenance vouched
// for by the package). Anti-rollback still enforced by the core. Returns
// the accepted version.
func (_self *KeepCore) LoadRulesetUnsigned(blob KeepRulesetBlob) (uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[KeepError](FfiConverterKeepError{}, func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_load_ruleset_unsigned(
			_pointer, FfiConverterKeepRulesetBlobINSTANCE.Lower(blob), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint64INSTANCE.Lift(_uniffiRV), nil
	}
}

// Open an output stream-filter bound to the active ruleset's exfil markers.
func (_self *KeepCore) OutputFilter() *KeepStreamFilter {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterKeepStreamFilterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_output_filter(
			_pointer, _uniffiStatus)
	}))
}

// Lowercase hex SHA-256 of a ruleset's canonical signing bytes (what a manifest's `artifactHash` names).
// A digest only: it says nothing about trust, and the canonical bytes stay in the core.
func (_self *KeepCore) RulesetDigest(blob KeepSignedRulesetBlob) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[KeepError](FfiConverterKeepError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_ruleset_digest(
				_pointer, FfiConverterKeepSignedRulesetBlobINSTANCE.Lower(blob), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *KeepCore) RulesetVersion() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_ruleset_version(
			_pointer, _uniffiStatus)
	}))
}

// The highest VERIFIED ruleset version loaded (0 = none): the anti-rollback floor for signed updates. A refresh
// client compares a published version against THIS, not the version in force, which may be an unsigned load.
func (_self *KeepCore) SignedFloor() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_signed_floor(
			_pointer, _uniffiStatus)
	}))
}

// The signing `kid`s currently trusted, sorted.
func (_self *KeepCore) TrustedKids() []string {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_trusted_kids(
				_pointer, _uniffiStatus),
		}
	}))
}

// Verify a ruleset manifest (compact JWS) under a trusted key and return its authenticated fields.
func (_self *KeepCore) VerifyManifest(jws string) (KeepManifestInfo, error) {
	_pointer := _self.ffiObject.incrementPointer("*KeepCore")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[KeepError](FfiConverterKeepError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepcore_verify_manifest(
				_pointer, FfiConverterStringINSTANCE.Lower(jws), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue KeepManifestInfo
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterKeepManifestInfoINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *KeepCore) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterKeepCore struct{}

var FfiConverterKeepCoreINSTANCE = FfiConverterKeepCore{}

func (c FfiConverterKeepCore) Lift(pointer unsafe.Pointer) *KeepCore {
	result := &KeepCore{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_k9o_keep_core_ffi_uniffi_fn_clone_keepcore(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_k9o_keep_core_ffi_uniffi_fn_free_keepcore(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*KeepCore).Destroy)
	return result
}

func (c FfiConverterKeepCore) Read(reader io.Reader) *KeepCore {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterKeepCore) Lower(value *KeepCore) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*KeepCore")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterKeepCore) Write(writer io.Writer, value *KeepCore) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerKeepCore struct{}

func (_ FfiDestroyerKeepCore) Destroy(value *KeepCore) {
	value.Destroy()
}

type KeepStreamFilterInterface interface {
	// Flush the residual buffer (also scanned) at end of stream.
	Finish() string
	// Feed a chunk; returns the cleared (redacted) prefix safe to emit now.
	Push(chunk string) string
	// Leak report (counts only — never the leaked values).
	Report() KeepLeakReport
}
type KeepStreamFilter struct {
	ffiObject FfiObject
}

// Flush the residual buffer (also scanned) at end of stream.
func (_self *KeepStreamFilter) Finish() string {
	_pointer := _self.ffiObject.incrementPointer("*KeepStreamFilter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepstreamfilter_finish(
				_pointer, _uniffiStatus),
		}
	}))
}

// Feed a chunk; returns the cleared (redacted) prefix safe to emit now.
func (_self *KeepStreamFilter) Push(chunk string) string {
	_pointer := _self.ffiObject.incrementPointer("*KeepStreamFilter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepstreamfilter_push(
				_pointer, FfiConverterStringINSTANCE.Lower(chunk), _uniffiStatus),
		}
	}))
}

// Leak report (counts only — never the leaked values).
func (_self *KeepStreamFilter) Report() KeepLeakReport {
	_pointer := _self.ffiObject.incrementPointer("*KeepStreamFilter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterKeepLeakReportINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_k9o_keep_core_ffi_uniffi_fn_method_keepstreamfilter_report(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *KeepStreamFilter) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterKeepStreamFilter struct{}

var FfiConverterKeepStreamFilterINSTANCE = FfiConverterKeepStreamFilter{}

func (c FfiConverterKeepStreamFilter) Lift(pointer unsafe.Pointer) *KeepStreamFilter {
	result := &KeepStreamFilter{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_k9o_keep_core_ffi_uniffi_fn_clone_keepstreamfilter(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_k9o_keep_core_ffi_uniffi_fn_free_keepstreamfilter(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*KeepStreamFilter).Destroy)
	return result
}

func (c FfiConverterKeepStreamFilter) Read(reader io.Reader) *KeepStreamFilter {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterKeepStreamFilter) Lower(value *KeepStreamFilter) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*KeepStreamFilter")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterKeepStreamFilter) Write(writer io.Writer, value *KeepStreamFilter) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerKeepStreamFilter struct{}

func (_ FfiDestroyerKeepStreamFilter) Destroy(value *KeepStreamFilter) {
	value.Destroy()
}

type KeepConfig struct {
	FailOpen bool
	Leet     bool
	// 0 = use the core default (3).
	DecodeDepth uint8
}

func (r *KeepConfig) Destroy() {
	FfiDestroyerBool{}.Destroy(r.FailOpen)
	FfiDestroyerBool{}.Destroy(r.Leet)
	FfiDestroyerUint8{}.Destroy(r.DecodeDepth)
}

type FfiConverterKeepConfig struct{}

var FfiConverterKeepConfigINSTANCE = FfiConverterKeepConfig{}

func (c FfiConverterKeepConfig) Lift(rb RustBufferI) KeepConfig {
	return LiftFromRustBuffer[KeepConfig](c, rb)
}

func (c FfiConverterKeepConfig) Read(reader io.Reader) KeepConfig {
	return KeepConfig{
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterUint8INSTANCE.Read(reader),
	}
}

func (c FfiConverterKeepConfig) Lower(value KeepConfig) C.RustBuffer {
	return LowerIntoRustBuffer[KeepConfig](c, value)
}

func (c FfiConverterKeepConfig) Write(writer io.Writer, value KeepConfig) {
	FfiConverterBoolINSTANCE.Write(writer, value.FailOpen)
	FfiConverterBoolINSTANCE.Write(writer, value.Leet)
	FfiConverterUint8INSTANCE.Write(writer, value.DecodeDepth)
}

type FfiDestroyerKeepConfig struct{}

func (_ FfiDestroyerKeepConfig) Destroy(value KeepConfig) {
	value.Destroy()
}

type KeepLeakReport struct {
	Emails  uint32
	Secrets uint32
	Markers uint32
	Total   uint32
}

func (r *KeepLeakReport) Destroy() {
	FfiDestroyerUint32{}.Destroy(r.Emails)
	FfiDestroyerUint32{}.Destroy(r.Secrets)
	FfiDestroyerUint32{}.Destroy(r.Markers)
	FfiDestroyerUint32{}.Destroy(r.Total)
}

type FfiConverterKeepLeakReport struct{}

var FfiConverterKeepLeakReportINSTANCE = FfiConverterKeepLeakReport{}

func (c FfiConverterKeepLeakReport) Lift(rb RustBufferI) KeepLeakReport {
	return LiftFromRustBuffer[KeepLeakReport](c, rb)
}

func (c FfiConverterKeepLeakReport) Read(reader io.Reader) KeepLeakReport {
	return KeepLeakReport{
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
	}
}

func (c FfiConverterKeepLeakReport) Lower(value KeepLeakReport) C.RustBuffer {
	return LowerIntoRustBuffer[KeepLeakReport](c, value)
}

func (c FfiConverterKeepLeakReport) Write(writer io.Writer, value KeepLeakReport) {
	FfiConverterUint32INSTANCE.Write(writer, value.Emails)
	FfiConverterUint32INSTANCE.Write(writer, value.Secrets)
	FfiConverterUint32INSTANCE.Write(writer, value.Markers)
	FfiConverterUint32INSTANCE.Write(writer, value.Total)
}

type FfiDestroyerKeepLeakReport struct{}

func (_ FfiDestroyerKeepLeakReport) Destroy(value KeepLeakReport) {
	value.Destroy()
}

// What a verified ruleset manifest says; every field is authenticated.
type KeepManifestInfo struct {
	RulesetVersion uint64
	SigningKeyKid  string
	ArtifactHash   *string
	ArtifactUrl    *string
	ValidUntil     *uint64
}

func (r *KeepManifestInfo) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.RulesetVersion)
	FfiDestroyerString{}.Destroy(r.SigningKeyKid)
	FfiDestroyerOptionalString{}.Destroy(r.ArtifactHash)
	FfiDestroyerOptionalString{}.Destroy(r.ArtifactUrl)
	FfiDestroyerOptionalUint64{}.Destroy(r.ValidUntil)
}

type FfiConverterKeepManifestInfo struct{}

var FfiConverterKeepManifestInfoINSTANCE = FfiConverterKeepManifestInfo{}

func (c FfiConverterKeepManifestInfo) Lift(rb RustBufferI) KeepManifestInfo {
	return LiftFromRustBuffer[KeepManifestInfo](c, rb)
}

func (c FfiConverterKeepManifestInfo) Read(reader io.Reader) KeepManifestInfo {
	return KeepManifestInfo{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterKeepManifestInfo) Lower(value KeepManifestInfo) C.RustBuffer {
	return LowerIntoRustBuffer[KeepManifestInfo](c, value)
}

func (c FfiConverterKeepManifestInfo) Write(writer io.Writer, value KeepManifestInfo) {
	FfiConverterUint64INSTANCE.Write(writer, value.RulesetVersion)
	FfiConverterStringINSTANCE.Write(writer, value.SigningKeyKid)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.ArtifactHash)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.ArtifactUrl)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.ValidUntil)
}

type FfiDestroyerKeepManifestInfo struct{}

func (_ FfiDestroyerKeepManifestInfo) Destroy(value KeepManifestInfo) {
	value.Destroy()
}

type KeepMatched struct {
	RuleId      string
	SpanStart   uint32
	SpanEnd     uint32
	Severity    string
	MatchedForm string
}

func (r *KeepMatched) Destroy() {
	FfiDestroyerString{}.Destroy(r.RuleId)
	FfiDestroyerUint32{}.Destroy(r.SpanStart)
	FfiDestroyerUint32{}.Destroy(r.SpanEnd)
	FfiDestroyerString{}.Destroy(r.Severity)
	FfiDestroyerString{}.Destroy(r.MatchedForm)
}

type FfiConverterKeepMatched struct{}

var FfiConverterKeepMatchedINSTANCE = FfiConverterKeepMatched{}

func (c FfiConverterKeepMatched) Lift(rb RustBufferI) KeepMatched {
	return LiftFromRustBuffer[KeepMatched](c, rb)
}

func (c FfiConverterKeepMatched) Read(reader io.Reader) KeepMatched {
	return KeepMatched{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterKeepMatched) Lower(value KeepMatched) C.RustBuffer {
	return LowerIntoRustBuffer[KeepMatched](c, value)
}

func (c FfiConverterKeepMatched) Write(writer io.Writer, value KeepMatched) {
	FfiConverterStringINSTANCE.Write(writer, value.RuleId)
	FfiConverterUint32INSTANCE.Write(writer, value.SpanStart)
	FfiConverterUint32INSTANCE.Write(writer, value.SpanEnd)
	FfiConverterStringINSTANCE.Write(writer, value.Severity)
	FfiConverterStringINSTANCE.Write(writer, value.MatchedForm)
}

type FfiDestroyerKeepMatched struct{}

func (_ FfiDestroyerKeepMatched) Destroy(value KeepMatched) {
	value.Destroy()
}

type KeepRule struct {
	Id      string
	Pattern string
	// One of "low" | "medium" | "high" | "critical".
	Severity string
}

func (r *KeepRule) Destroy() {
	FfiDestroyerString{}.Destroy(r.Id)
	FfiDestroyerString{}.Destroy(r.Pattern)
	FfiDestroyerString{}.Destroy(r.Severity)
}

type FfiConverterKeepRule struct{}

var FfiConverterKeepRuleINSTANCE = FfiConverterKeepRule{}

func (c FfiConverterKeepRule) Lift(rb RustBufferI) KeepRule {
	return LiftFromRustBuffer[KeepRule](c, rb)
}

func (c FfiConverterKeepRule) Read(reader io.Reader) KeepRule {
	return KeepRule{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterKeepRule) Lower(value KeepRule) C.RustBuffer {
	return LowerIntoRustBuffer[KeepRule](c, value)
}

func (c FfiConverterKeepRule) Write(writer io.Writer, value KeepRule) {
	FfiConverterStringINSTANCE.Write(writer, value.Id)
	FfiConverterStringINSTANCE.Write(writer, value.Pattern)
	FfiConverterStringINSTANCE.Write(writer, value.Severity)
}

type FfiDestroyerKeepRule struct{}

func (_ FfiDestroyerKeepRule) Destroy(value KeepRule) {
	value.Destroy()
}

type KeepRulesetBlob struct {
	RulesetVersion uint64
	Rules          []KeepRule
}

func (r *KeepRulesetBlob) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.RulesetVersion)
	FfiDestroyerSequenceKeepRule{}.Destroy(r.Rules)
}

type FfiConverterKeepRulesetBlob struct{}

var FfiConverterKeepRulesetBlobINSTANCE = FfiConverterKeepRulesetBlob{}

func (c FfiConverterKeepRulesetBlob) Lift(rb RustBufferI) KeepRulesetBlob {
	return LiftFromRustBuffer[KeepRulesetBlob](c, rb)
}

func (c FfiConverterKeepRulesetBlob) Read(reader io.Reader) KeepRulesetBlob {
	return KeepRulesetBlob{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterSequenceKeepRuleINSTANCE.Read(reader),
	}
}

func (c FfiConverterKeepRulesetBlob) Lower(value KeepRulesetBlob) C.RustBuffer {
	return LowerIntoRustBuffer[KeepRulesetBlob](c, value)
}

func (c FfiConverterKeepRulesetBlob) Write(writer io.Writer, value KeepRulesetBlob) {
	FfiConverterUint64INSTANCE.Write(writer, value.RulesetVersion)
	FfiConverterSequenceKeepRuleINSTANCE.Write(writer, value.Rules)
}

type FfiDestroyerKeepRulesetBlob struct{}

func (_ FfiDestroyerKeepRulesetBlob) Destroy(value KeepRulesetBlob) {
	value.Destroy()
}

// A signed ruleset as fetched from Barbican. `signing_key_kid` and `signature` are checked by the core.
type KeepSignedRulesetBlob struct {
	RulesetVersion uint64
	Rules          []KeepRule
	SigningKeyKid  string
	Signature      []byte
	CreatedAt      int64
}

func (r *KeepSignedRulesetBlob) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.RulesetVersion)
	FfiDestroyerSequenceKeepRule{}.Destroy(r.Rules)
	FfiDestroyerString{}.Destroy(r.SigningKeyKid)
	FfiDestroyerBytes{}.Destroy(r.Signature)
	FfiDestroyerInt64{}.Destroy(r.CreatedAt)
}

type FfiConverterKeepSignedRulesetBlob struct{}

var FfiConverterKeepSignedRulesetBlobINSTANCE = FfiConverterKeepSignedRulesetBlob{}

func (c FfiConverterKeepSignedRulesetBlob) Lift(rb RustBufferI) KeepSignedRulesetBlob {
	return LiftFromRustBuffer[KeepSignedRulesetBlob](c, rb)
}

func (c FfiConverterKeepSignedRulesetBlob) Read(reader io.Reader) KeepSignedRulesetBlob {
	return KeepSignedRulesetBlob{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterSequenceKeepRuleINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterBytesINSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
	}
}

func (c FfiConverterKeepSignedRulesetBlob) Lower(value KeepSignedRulesetBlob) C.RustBuffer {
	return LowerIntoRustBuffer[KeepSignedRulesetBlob](c, value)
}

func (c FfiConverterKeepSignedRulesetBlob) Write(writer io.Writer, value KeepSignedRulesetBlob) {
	FfiConverterUint64INSTANCE.Write(writer, value.RulesetVersion)
	FfiConverterSequenceKeepRuleINSTANCE.Write(writer, value.Rules)
	FfiConverterStringINSTANCE.Write(writer, value.SigningKeyKid)
	FfiConverterBytesINSTANCE.Write(writer, value.Signature)
	FfiConverterInt64INSTANCE.Write(writer, value.CreatedAt)
}

type FfiDestroyerKeepSignedRulesetBlob struct{}

func (_ FfiDestroyerKeepSignedRulesetBlob) Destroy(value KeepSignedRulesetBlob) {
	value.Destroy()
}

type KeepVerdict struct {
	Decision          string
	Reason            string
	Matched           []KeepMatched
	SchemaValid       bool
	WaypointClassHint *string
	FailMode          *string
	RulesetVersion    uint64
	CoreVersion       string
}

func (r *KeepVerdict) Destroy() {
	FfiDestroyerString{}.Destroy(r.Decision)
	FfiDestroyerString{}.Destroy(r.Reason)
	FfiDestroyerSequenceKeepMatched{}.Destroy(r.Matched)
	FfiDestroyerBool{}.Destroy(r.SchemaValid)
	FfiDestroyerOptionalString{}.Destroy(r.WaypointClassHint)
	FfiDestroyerOptionalString{}.Destroy(r.FailMode)
	FfiDestroyerUint64{}.Destroy(r.RulesetVersion)
	FfiDestroyerString{}.Destroy(r.CoreVersion)
}

type FfiConverterKeepVerdict struct{}

var FfiConverterKeepVerdictINSTANCE = FfiConverterKeepVerdict{}

func (c FfiConverterKeepVerdict) Lift(rb RustBufferI) KeepVerdict {
	return LiftFromRustBuffer[KeepVerdict](c, rb)
}

func (c FfiConverterKeepVerdict) Read(reader io.Reader) KeepVerdict {
	return KeepVerdict{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterSequenceKeepMatchedINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterKeepVerdict) Lower(value KeepVerdict) C.RustBuffer {
	return LowerIntoRustBuffer[KeepVerdict](c, value)
}

func (c FfiConverterKeepVerdict) Write(writer io.Writer, value KeepVerdict) {
	FfiConverterStringINSTANCE.Write(writer, value.Decision)
	FfiConverterStringINSTANCE.Write(writer, value.Reason)
	FfiConverterSequenceKeepMatchedINSTANCE.Write(writer, value.Matched)
	FfiConverterBoolINSTANCE.Write(writer, value.SchemaValid)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.WaypointClassHint)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.FailMode)
	FfiConverterUint64INSTANCE.Write(writer, value.RulesetVersion)
	FfiConverterStringINSTANCE.Write(writer, value.CoreVersion)
}

type FfiDestroyerKeepVerdict struct{}

func (_ FfiDestroyerKeepVerdict) Destroy(value KeepVerdict) {
	value.Destroy()
}

type KeepError struct {
	err error
}

// Convience method to turn *KeepError into error
// Avoiding treating nil pointer as non nil error interface
func (err *KeepError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err KeepError) Error() string {
	return fmt.Sprintf("KeepError: %s", err.err.Error())
}

func (err KeepError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrKeepErrorRulesetInvalid = fmt.Errorf("KeepErrorRulesetInvalid")

// Variant structs
type KeepErrorRulesetInvalid struct {
	Reason string
}

func NewKeepErrorRulesetInvalid(
	reason string,
) *KeepError {
	return &KeepError{err: &KeepErrorRulesetInvalid{
		Reason: reason}}
}

func (e KeepErrorRulesetInvalid) destroy() {
	FfiDestroyerString{}.Destroy(e.Reason)
}

func (err KeepErrorRulesetInvalid) Error() string {
	return fmt.Sprint("RulesetInvalid",
		": ",

		"Reason=",
		err.Reason,
	)
}

func (self KeepErrorRulesetInvalid) Is(target error) bool {
	return target == ErrKeepErrorRulesetInvalid
}

type FfiConverterKeepError struct{}

var FfiConverterKeepErrorINSTANCE = FfiConverterKeepError{}

func (c FfiConverterKeepError) Lift(eb RustBufferI) *KeepError {
	return LiftFromRustBuffer[*KeepError](c, eb)
}

func (c FfiConverterKeepError) Lower(value *KeepError) C.RustBuffer {
	return LowerIntoRustBuffer[*KeepError](c, value)
}

func (c FfiConverterKeepError) Read(reader io.Reader) *KeepError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &KeepError{&KeepErrorRulesetInvalid{
			Reason: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterKeepError.Read()", errorID))
	}
}

func (c FfiConverterKeepError) Write(writer io.Writer, value *KeepError) {
	switch variantValue := value.err.(type) {
	case *KeepErrorRulesetInvalid:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Reason)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterKeepError.Write", value))
	}
}

type FfiDestroyerKeepError struct{}

func (_ FfiDestroyerKeepError) Destroy(value *KeepError) {
	switch variantValue := value.err.(type) {
	case KeepErrorRulesetInvalid:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerKeepError.Destroy", value))
	}
}

type FfiConverterOptionalUint64 struct{}

var FfiConverterOptionalUint64INSTANCE = FfiConverterOptionalUint64{}

func (c FfiConverterOptionalUint64) Lift(rb RustBufferI) *uint64 {
	return LiftFromRustBuffer[*uint64](c, rb)
}

func (_ FfiConverterOptionalUint64) Read(reader io.Reader) *uint64 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint64INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint64) Lower(value *uint64) C.RustBuffer {
	return LowerIntoRustBuffer[*uint64](c, value)
}

func (_ FfiConverterOptionalUint64) Write(writer io.Writer, value *uint64) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint64 struct{}

func (_ FfiDestroyerOptionalUint64) Destroy(value *uint64) {
	if value != nil {
		FfiDestroyerUint64{}.Destroy(*value)
	}
}

type FfiConverterOptionalString struct{}

var FfiConverterOptionalStringINSTANCE = FfiConverterOptionalString{}

func (c FfiConverterOptionalString) Lift(rb RustBufferI) *string {
	return LiftFromRustBuffer[*string](c, rb)
}

func (_ FfiConverterOptionalString) Read(reader io.Reader) *string {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterStringINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalString) Lower(value *string) C.RustBuffer {
	return LowerIntoRustBuffer[*string](c, value)
}

func (_ FfiConverterOptionalString) Write(writer io.Writer, value *string) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalString struct{}

func (_ FfiDestroyerOptionalString) Destroy(value *string) {
	if value != nil {
		FfiDestroyerString{}.Destroy(*value)
	}
}

type FfiConverterSequenceString struct{}

var FfiConverterSequenceStringINSTANCE = FfiConverterSequenceString{}

func (c FfiConverterSequenceString) Lift(rb RustBufferI) []string {
	return LiftFromRustBuffer[[]string](c, rb)
}

func (c FfiConverterSequenceString) Read(reader io.Reader) []string {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]string, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterStringINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceString) Lower(value []string) C.RustBuffer {
	return LowerIntoRustBuffer[[]string](c, value)
}

func (c FfiConverterSequenceString) Write(writer io.Writer, value []string) {
	if len(value) > math.MaxInt32 {
		panic("[]string is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterStringINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceString struct{}

func (FfiDestroyerSequenceString) Destroy(sequence []string) {
	for _, value := range sequence {
		FfiDestroyerString{}.Destroy(value)
	}
}

type FfiConverterSequenceKeepMatched struct{}

var FfiConverterSequenceKeepMatchedINSTANCE = FfiConverterSequenceKeepMatched{}

func (c FfiConverterSequenceKeepMatched) Lift(rb RustBufferI) []KeepMatched {
	return LiftFromRustBuffer[[]KeepMatched](c, rb)
}

func (c FfiConverterSequenceKeepMatched) Read(reader io.Reader) []KeepMatched {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]KeepMatched, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterKeepMatchedINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceKeepMatched) Lower(value []KeepMatched) C.RustBuffer {
	return LowerIntoRustBuffer[[]KeepMatched](c, value)
}

func (c FfiConverterSequenceKeepMatched) Write(writer io.Writer, value []KeepMatched) {
	if len(value) > math.MaxInt32 {
		panic("[]KeepMatched is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterKeepMatchedINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceKeepMatched struct{}

func (FfiDestroyerSequenceKeepMatched) Destroy(sequence []KeepMatched) {
	for _, value := range sequence {
		FfiDestroyerKeepMatched{}.Destroy(value)
	}
}

type FfiConverterSequenceKeepRule struct{}

var FfiConverterSequenceKeepRuleINSTANCE = FfiConverterSequenceKeepRule{}

func (c FfiConverterSequenceKeepRule) Lift(rb RustBufferI) []KeepRule {
	return LiftFromRustBuffer[[]KeepRule](c, rb)
}

func (c FfiConverterSequenceKeepRule) Read(reader io.Reader) []KeepRule {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]KeepRule, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterKeepRuleINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceKeepRule) Lower(value []KeepRule) C.RustBuffer {
	return LowerIntoRustBuffer[[]KeepRule](c, value)
}

func (c FfiConverterSequenceKeepRule) Write(writer io.Writer, value []KeepRule) {
	if len(value) > math.MaxInt32 {
		panic("[]KeepRule is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterKeepRuleINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceKeepRule struct{}

func (FfiDestroyerSequenceKeepRule) Destroy(sequence []KeepRule) {
	for _, value := range sequence {
		FfiDestroyerKeepRule{}.Destroy(value)
	}
}

// Whether this build trusts the published TEST root. Release CI asserts `false` on the shipped artifact.
func UsesTestTrustRoot() bool {
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_k9o_keep_core_ffi_uniffi_fn_func_uses_test_trust_root(_uniffiStatus)
	}))
}
