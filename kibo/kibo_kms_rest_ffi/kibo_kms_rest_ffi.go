package kibo_kms_rest_ffi

// #include <kibo_kms_rest_ffi.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"runtime/cgo"
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
		C.ffi_kibo_kms_rest_ffi_rustbuffer_free(cb.inner, status)
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
		return C.ffi_kibo_kms_rest_ffi_rustbuffer_from_bytes(foreign, status)
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
		return C.ffi_kibo_kms_rest_ffi_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("kibo_kms_rest_ffi: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_kibo_kms_rest_ffi_checksum_func_zk_decrypt()
		})
		if checksum != 8128 {
			// If this happens try cleaning and rebuilding your project
			panic("kibo_kms_rest_ffi: uniffi_kibo_kms_rest_ffi_checksum_func_zk_decrypt: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_kibo_kms_rest_ffi_checksum_func_zk_decrypt_blocking()
		})
		if checksum != 63808 {
			// If this happens try cleaning and rebuilding your project
			panic("kibo_kms_rest_ffi: uniffi_kibo_kms_rest_ffi_checksum_func_zk_decrypt_blocking: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_kibo_kms_rest_ffi_checksum_func_zk_encrypt()
		})
		if checksum != 65185 {
			// If this happens try cleaning and rebuilding your project
			panic("kibo_kms_rest_ffi: uniffi_kibo_kms_rest_ffi_checksum_func_zk_encrypt: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_kibo_kms_rest_ffi_checksum_func_zk_encrypt_blocking()
		})
		if checksum != 54723 {
			// If this happens try cleaning and rebuilding your project
			panic("kibo_kms_rest_ffi: uniffi_kibo_kms_rest_ffi_checksum_func_zk_encrypt_blocking: UniFFI API checksum mismatch")
		}
	}
}

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

// All inputs to `zk_decrypt`.
type DecryptInput struct {
	// Customer KMS credential. Must be valid for the wrapped-DEK unwrap
	// (or oracle ciphertext decrypt). Zeroized after use.
	KmsCred FfiCredential
	// Provider-specific initialization (region for AWS, address for Vault).
	ProviderConfig FfiProviderConfig
	// The canonical (`b"KIBO"`) envelope returned by a prior `zk_encrypt` call.
	Envelope []byte
	// Strategy used at encrypt — MUST be re-supplied by the caller. Strategy
	// is not stored in the canonical envelope header (it is an AAD-bound
	// dimension in `AadContext`; the KMS verifies it matches at decrypt). The
	// caller always knows the strategy used at encrypt; it is not inferred from
	// the envelope bytes.
	Strategy FfiStrategy
	// Device attestation tier — MUST be re-supplied by the caller; never
	// stored in or recovered from the envelope. `attestation_tier` is one of
	// the three reserved AAD dimensions (`kibo:attestation_tier`) bound at
	// encrypt time; the AEAD tag verification fails if the re-supplied value
	// differs from the one used at encrypt. Symmetric with `EncryptInput`.
	// Source from deployment/project config (fixed per deployment, not per-op).
	AttestationTier FfiAttestationTier
	// Encryption context — MUST be re-supplied by the caller; never read
	// from the envelope (anti-envelope-substitution discipline).
	EncryptionContext []FfiKvPair
}

func (r *DecryptInput) Destroy() {
	FfiDestroyerFfiCredential{}.Destroy(r.KmsCred)
	FfiDestroyerFfiProviderConfig{}.Destroy(r.ProviderConfig)
	FfiDestroyerBytes{}.Destroy(r.Envelope)
	FfiDestroyerFfiStrategy{}.Destroy(r.Strategy)
	FfiDestroyerFfiAttestationTier{}.Destroy(r.AttestationTier)
	FfiDestroyerSequenceFfiKvPair{}.Destroy(r.EncryptionContext)
}

type FfiConverterDecryptInput struct{}

var FfiConverterDecryptInputINSTANCE = FfiConverterDecryptInput{}

func (c FfiConverterDecryptInput) Lift(rb RustBufferI) DecryptInput {
	return LiftFromRustBuffer[DecryptInput](c, rb)
}

func (c FfiConverterDecryptInput) Read(reader io.Reader) DecryptInput {
	return DecryptInput{
		FfiConverterFfiCredentialINSTANCE.Read(reader),
		FfiConverterFfiProviderConfigINSTANCE.Read(reader),
		FfiConverterBytesINSTANCE.Read(reader),
		FfiConverterFfiStrategyINSTANCE.Read(reader),
		FfiConverterFfiAttestationTierINSTANCE.Read(reader),
		FfiConverterSequenceFfiKvPairINSTANCE.Read(reader),
	}
}

func (c FfiConverterDecryptInput) Lower(value DecryptInput) C.RustBuffer {
	return LowerIntoRustBuffer[DecryptInput](c, value)
}

func (c FfiConverterDecryptInput) Write(writer io.Writer, value DecryptInput) {
	FfiConverterFfiCredentialINSTANCE.Write(writer, value.KmsCred)
	FfiConverterFfiProviderConfigINSTANCE.Write(writer, value.ProviderConfig)
	FfiConverterBytesINSTANCE.Write(writer, value.Envelope)
	FfiConverterFfiStrategyINSTANCE.Write(writer, value.Strategy)
	FfiConverterFfiAttestationTierINSTANCE.Write(writer, value.AttestationTier)
	FfiConverterSequenceFfiKvPairINSTANCE.Write(writer, value.EncryptionContext)
}

type FfiDestroyerDecryptInput struct{}

func (_ FfiDestroyerDecryptInput) Destroy(value DecryptInput) {
	value.Destroy()
}

// Output of `zk_decrypt`.
type DecryptOutput struct {
	// Recovered plaintext bytes.
	Plaintext []byte
}

func (r *DecryptOutput) Destroy() {
	FfiDestroyerBytes{}.Destroy(r.Plaintext)
}

type FfiConverterDecryptOutput struct{}

var FfiConverterDecryptOutputINSTANCE = FfiConverterDecryptOutput{}

func (c FfiConverterDecryptOutput) Lift(rb RustBufferI) DecryptOutput {
	return LiftFromRustBuffer[DecryptOutput](c, rb)
}

func (c FfiConverterDecryptOutput) Read(reader io.Reader) DecryptOutput {
	return DecryptOutput{
		FfiConverterBytesINSTANCE.Read(reader),
	}
}

func (c FfiConverterDecryptOutput) Lower(value DecryptOutput) C.RustBuffer {
	return LowerIntoRustBuffer[DecryptOutput](c, value)
}

func (c FfiConverterDecryptOutput) Write(writer io.Writer, value DecryptOutput) {
	FfiConverterBytesINSTANCE.Write(writer, value.Plaintext)
}

type FfiDestroyerDecryptOutput struct{}

func (_ FfiDestroyerDecryptOutput) Destroy(value DecryptOutput) {
	value.Destroy()
}

// All inputs to `zk_encrypt`.
type EncryptInput struct {
	// The customer KMS provider. Must match the compiled-in provider feature.
	Provider KmsProvider
	// Custody posture (Byok or Hyok — PlatformManaged excluded).
	Posture FfiPosture
	// Explicit strategy pin. `None` → `LocalEnvelope` (posture default).
	Strategy *FfiStrategy
	// Oracle routing mode. `None` → `Brokered` (default).
	OracleMode *FfiOracleMode
	// Customer KMS credential. Customer-issued short-lived token; never
	// a platform KEK. Zeroized after use.
	KmsCred FfiCredential
	// Customer KMS key version reference.
	KeyRef FfiKeyRef
	// Provider-specific initialization (region for AWS, address for Vault).
	ProviderConfig FfiProviderConfig
	// Device attestation tier bound into the AAD.
	AttestationTier FfiAttestationTier
	// User-supplied encryption context — key/value pairs bound into the AAD
	// and re-supplied byte-for-byte at decrypt. `kibo:` prefix is reserved.
	EncryptionContext []FfiKvPair
	// Plaintext bytes to encrypt.
	Plaintext []byte
	// Project-declared crypto-posture floor. `None` = permissive default.
	PosturePolicy *FfiPosturePolicy
}

func (r *EncryptInput) Destroy() {
	FfiDestroyerKmsProvider{}.Destroy(r.Provider)
	FfiDestroyerFfiPosture{}.Destroy(r.Posture)
	FfiDestroyerOptionalFfiStrategy{}.Destroy(r.Strategy)
	FfiDestroyerOptionalFfiOracleMode{}.Destroy(r.OracleMode)
	FfiDestroyerFfiCredential{}.Destroy(r.KmsCred)
	FfiDestroyerFfiKeyRef{}.Destroy(r.KeyRef)
	FfiDestroyerFfiProviderConfig{}.Destroy(r.ProviderConfig)
	FfiDestroyerFfiAttestationTier{}.Destroy(r.AttestationTier)
	FfiDestroyerSequenceFfiKvPair{}.Destroy(r.EncryptionContext)
	FfiDestroyerBytes{}.Destroy(r.Plaintext)
	FfiDestroyerOptionalFfiPosturePolicy{}.Destroy(r.PosturePolicy)
}

type FfiConverterEncryptInput struct{}

var FfiConverterEncryptInputINSTANCE = FfiConverterEncryptInput{}

func (c FfiConverterEncryptInput) Lift(rb RustBufferI) EncryptInput {
	return LiftFromRustBuffer[EncryptInput](c, rb)
}

func (c FfiConverterEncryptInput) Read(reader io.Reader) EncryptInput {
	return EncryptInput{
		FfiConverterKmsProviderINSTANCE.Read(reader),
		FfiConverterFfiPostureINSTANCE.Read(reader),
		FfiConverterOptionalFfiStrategyINSTANCE.Read(reader),
		FfiConverterOptionalFfiOracleModeINSTANCE.Read(reader),
		FfiConverterFfiCredentialINSTANCE.Read(reader),
		FfiConverterFfiKeyRefINSTANCE.Read(reader),
		FfiConverterFfiProviderConfigINSTANCE.Read(reader),
		FfiConverterFfiAttestationTierINSTANCE.Read(reader),
		FfiConverterSequenceFfiKvPairINSTANCE.Read(reader),
		FfiConverterBytesINSTANCE.Read(reader),
		FfiConverterOptionalFfiPosturePolicyINSTANCE.Read(reader),
	}
}

func (c FfiConverterEncryptInput) Lower(value EncryptInput) C.RustBuffer {
	return LowerIntoRustBuffer[EncryptInput](c, value)
}

func (c FfiConverterEncryptInput) Write(writer io.Writer, value EncryptInput) {
	FfiConverterKmsProviderINSTANCE.Write(writer, value.Provider)
	FfiConverterFfiPostureINSTANCE.Write(writer, value.Posture)
	FfiConverterOptionalFfiStrategyINSTANCE.Write(writer, value.Strategy)
	FfiConverterOptionalFfiOracleModeINSTANCE.Write(writer, value.OracleMode)
	FfiConverterFfiCredentialINSTANCE.Write(writer, value.KmsCred)
	FfiConverterFfiKeyRefINSTANCE.Write(writer, value.KeyRef)
	FfiConverterFfiProviderConfigINSTANCE.Write(writer, value.ProviderConfig)
	FfiConverterFfiAttestationTierINSTANCE.Write(writer, value.AttestationTier)
	FfiConverterSequenceFfiKvPairINSTANCE.Write(writer, value.EncryptionContext)
	FfiConverterBytesINSTANCE.Write(writer, value.Plaintext)
	FfiConverterOptionalFfiPosturePolicyINSTANCE.Write(writer, value.PosturePolicy)
}

type FfiDestroyerEncryptInput struct{}

func (_ FfiDestroyerEncryptInput) Destroy(value EncryptInput) {
	value.Destroy()
}

// Output of `zk_encrypt`.
type EncryptOutput struct {
	// ZKFE self-describing envelope. Pass opaque to `zk_decrypt`.
	Envelope []byte
}

func (r *EncryptOutput) Destroy() {
	FfiDestroyerBytes{}.Destroy(r.Envelope)
}

type FfiConverterEncryptOutput struct{}

var FfiConverterEncryptOutputINSTANCE = FfiConverterEncryptOutput{}

func (c FfiConverterEncryptOutput) Lift(rb RustBufferI) EncryptOutput {
	return LiftFromRustBuffer[EncryptOutput](c, rb)
}

func (c FfiConverterEncryptOutput) Read(reader io.Reader) EncryptOutput {
	return EncryptOutput{
		FfiConverterBytesINSTANCE.Read(reader),
	}
}

func (c FfiConverterEncryptOutput) Lower(value EncryptOutput) C.RustBuffer {
	return LowerIntoRustBuffer[EncryptOutput](c, value)
}

func (c FfiConverterEncryptOutput) Write(writer io.Writer, value EncryptOutput) {
	FfiConverterBytesINSTANCE.Write(writer, value.Envelope)
}

type FfiDestroyerEncryptOutput struct{}

func (_ FfiDestroyerEncryptOutput) Destroy(value EncryptOutput) {
	value.Destroy()
}

// Acceptable oracle-mode set for the allowed_oracle_modes policy dial.
type FfiAllowedOracleModes struct {
	Brokered bool
	Direct   bool
}

func (r *FfiAllowedOracleModes) Destroy() {
	FfiDestroyerBool{}.Destroy(r.Brokered)
	FfiDestroyerBool{}.Destroy(r.Direct)
}

type FfiConverterFfiAllowedOracleModes struct{}

var FfiConverterFfiAllowedOracleModesINSTANCE = FfiConverterFfiAllowedOracleModes{}

func (c FfiConverterFfiAllowedOracleModes) Lift(rb RustBufferI) FfiAllowedOracleModes {
	return LiftFromRustBuffer[FfiAllowedOracleModes](c, rb)
}

func (c FfiConverterFfiAllowedOracleModes) Read(reader io.Reader) FfiAllowedOracleModes {
	return FfiAllowedOracleModes{
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterFfiAllowedOracleModes) Lower(value FfiAllowedOracleModes) C.RustBuffer {
	return LowerIntoRustBuffer[FfiAllowedOracleModes](c, value)
}

func (c FfiConverterFfiAllowedOracleModes) Write(writer io.Writer, value FfiAllowedOracleModes) {
	FfiConverterBoolINSTANCE.Write(writer, value.Brokered)
	FfiConverterBoolINSTANCE.Write(writer, value.Direct)
}

type FfiDestroyerFfiAllowedOracleModes struct{}

func (_ FfiDestroyerFfiAllowedOracleModes) Destroy(value FfiAllowedOracleModes) {
	value.Destroy()
}

// Short-lived customer KMS credential — the edge device's federated token.
// Zeroized on drop in the engine (mapped to `FederatedToken`). The `secret`
// is never logged or returned in any output or error.
type FfiCredential struct {
	// Opaque credential value (e.g., STS session token, GCP OAuth bearer,
	// Vault token). Treated as secret material; zeroized after use.
	Secret string
	// Absolute expiry as Unix seconds. The caller is responsible for refresh.
	ExpiresAtUnixSecs uint64
}

func (r *FfiCredential) Destroy() {
	FfiDestroyerString{}.Destroy(r.Secret)
	FfiDestroyerUint64{}.Destroy(r.ExpiresAtUnixSecs)
}

type FfiConverterFfiCredential struct{}

var FfiConverterFfiCredentialINSTANCE = FfiConverterFfiCredential{}

func (c FfiConverterFfiCredential) Lift(rb RustBufferI) FfiCredential {
	return LiftFromRustBuffer[FfiCredential](c, rb)
}

func (c FfiConverterFfiCredential) Read(reader io.Reader) FfiCredential {
	return FfiCredential{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterFfiCredential) Lower(value FfiCredential) C.RustBuffer {
	return LowerIntoRustBuffer[FfiCredential](c, value)
}

func (c FfiConverterFfiCredential) Write(writer io.Writer, value FfiCredential) {
	FfiConverterStringINSTANCE.Write(writer, value.Secret)
	FfiConverterUint64INSTANCE.Write(writer, value.ExpiresAtUnixSecs)
}

type FfiDestroyerFfiCredential struct{}

func (_ FfiDestroyerFfiCredential) Destroy(value FfiCredential) {
	value.Destroy()
}

// Key version reference identifying the customer KMS key. Maps directly to
// `kibo_core::types::KeyVersionRef`; `external_ref` format is provider-specific:
// - AWS: full key ARN (`arn:aws:kms:<region>:<acct>:key/<id>`)
// - GCP: fully-qualified CryptoKey resource name
// - Azure: versioned key identifier URL (`https://<vault>.vault.azure.net/keys/<name>/<ver>`)
// - Vault: `<mount>/<key_name>:<version>` (e.g. `transit/mykey:3`)
type FfiKeyRef struct {
	// Monotone version counter across all KEK rotations and re-key transitions.
	Version uint64
	// Provider-formatted opaque identifier (see above).
	ExternalRef string
}

func (r *FfiKeyRef) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Version)
	FfiDestroyerString{}.Destroy(r.ExternalRef)
}

type FfiConverterFfiKeyRef struct{}

var FfiConverterFfiKeyRefINSTANCE = FfiConverterFfiKeyRef{}

func (c FfiConverterFfiKeyRef) Lift(rb RustBufferI) FfiKeyRef {
	return LiftFromRustBuffer[FfiKeyRef](c, rb)
}

func (c FfiConverterFfiKeyRef) Read(reader io.Reader) FfiKeyRef {
	return FfiKeyRef{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterFfiKeyRef) Lower(value FfiKeyRef) C.RustBuffer {
	return LowerIntoRustBuffer[FfiKeyRef](c, value)
}

func (c FfiConverterFfiKeyRef) Write(writer io.Writer, value FfiKeyRef) {
	FfiConverterUint64INSTANCE.Write(writer, value.Version)
	FfiConverterStringINSTANCE.Write(writer, value.ExternalRef)
}

type FfiDestroyerFfiKeyRef struct{}

func (_ FfiDestroyerFfiKeyRef) Destroy(value FfiKeyRef) {
	value.Destroy()
}

// A single encryption-context key/value pair.
//
// `Vec<(String, String)>` cannot cross the uniffi boundary (tuples are
// unsupported); this named record is the wire-compatible replacement.
// JSON serialization: `{"key": "...", "value": "..."}` (object, not array).
type FfiKvPair struct {
	Key   string
	Value string
}

func (r *FfiKvPair) Destroy() {
	FfiDestroyerString{}.Destroy(r.Key)
	FfiDestroyerString{}.Destroy(r.Value)
}

type FfiConverterFfiKvPair struct{}

var FfiConverterFfiKvPairINSTANCE = FfiConverterFfiKvPair{}

func (c FfiConverterFfiKvPair) Lift(rb RustBufferI) FfiKvPair {
	return LiftFromRustBuffer[FfiKvPair](c, rb)
}

func (c FfiConverterFfiKvPair) Read(reader io.Reader) FfiKvPair {
	return FfiKvPair{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterFfiKvPair) Lower(value FfiKvPair) C.RustBuffer {
	return LowerIntoRustBuffer[FfiKvPair](c, value)
}

func (c FfiConverterFfiKvPair) Write(writer io.Writer, value FfiKvPair) {
	FfiConverterStringINSTANCE.Write(writer, value.Key)
	FfiConverterStringINSTANCE.Write(writer, value.Value)
}

type FfiDestroyerFfiKvPair struct{}

func (_ FfiDestroyerFfiKvPair) Destroy(value FfiKvPair) {
	value.Destroy()
}

// Project-declared crypto-posture floor. Two sub-dials:
// `min_custody_posture` is a ladder-floor (`PlatformManaged < Byok < Hyok`);
// `allowed_oracle_modes` is an acceptable-set (`⊆ {brokered, direct}`).
//
// Default = most permissive (`PlatformManaged` floor + both oracle modes).
// `min_custody_posture: None` → `PlatformManaged` (admits every posture).
// `allowed_oracle_modes: None` → both modes allowed.
type FfiPosturePolicy struct {
	// Minimum custody posture on the sovereignty ladder. `None` = no floor.
	// Note: `None` and `Some(PlatformManaged)` are equivalent here.
	MinCustodyPosture *FfiMinPosture
	// Permitted oracle modes. `None` = both modes allowed.
	AllowedOracleModes *FfiAllowedOracleModes
}

func (r *FfiPosturePolicy) Destroy() {
	FfiDestroyerOptionalFfiMinPosture{}.Destroy(r.MinCustodyPosture)
	FfiDestroyerOptionalFfiAllowedOracleModes{}.Destroy(r.AllowedOracleModes)
}

type FfiConverterFfiPosturePolicy struct{}

var FfiConverterFfiPosturePolicyINSTANCE = FfiConverterFfiPosturePolicy{}

func (c FfiConverterFfiPosturePolicy) Lift(rb RustBufferI) FfiPosturePolicy {
	return LiftFromRustBuffer[FfiPosturePolicy](c, rb)
}

func (c FfiConverterFfiPosturePolicy) Read(reader io.Reader) FfiPosturePolicy {
	return FfiPosturePolicy{
		FfiConverterOptionalFfiMinPostureINSTANCE.Read(reader),
		FfiConverterOptionalFfiAllowedOracleModesINSTANCE.Read(reader),
	}
}

func (c FfiConverterFfiPosturePolicy) Lower(value FfiPosturePolicy) C.RustBuffer {
	return LowerIntoRustBuffer[FfiPosturePolicy](c, value)
}

func (c FfiConverterFfiPosturePolicy) Write(writer io.Writer, value FfiPosturePolicy) {
	FfiConverterOptionalFfiMinPostureINSTANCE.Write(writer, value.MinCustodyPosture)
	FfiConverterOptionalFfiAllowedOracleModesINSTANCE.Write(writer, value.AllowedOracleModes)
}

type FfiDestroyerFfiPosturePolicy struct{}

func (_ FfiDestroyerFfiPosturePolicy) Destroy(value FfiPosturePolicy) {
	value.Destroy()
}

// Provider-specific initialization parameters. Only the field relevant to
// the compiled-in provider is consulted; others are ignored.
type FfiProviderConfig struct {
	// AWS region (e.g. `us-east-1`). Falls back to ARN-extraction from
	// `key_ref.external_ref` if `None` (ARN format only).
	AwsRegion *string
	// Vault base address (e.g. `https://vault.example.com:8200`). Required
	// when the compiled-in provider is `kms-vault`.
	VaultAddress *string
}

func (r *FfiProviderConfig) Destroy() {
	FfiDestroyerOptionalString{}.Destroy(r.AwsRegion)
	FfiDestroyerOptionalString{}.Destroy(r.VaultAddress)
}

type FfiConverterFfiProviderConfig struct{}

var FfiConverterFfiProviderConfigINSTANCE = FfiConverterFfiProviderConfig{}

func (c FfiConverterFfiProviderConfig) Lift(rb RustBufferI) FfiProviderConfig {
	return LiftFromRustBuffer[FfiProviderConfig](c, rb)
}

func (c FfiConverterFfiProviderConfig) Read(reader io.Reader) FfiProviderConfig {
	return FfiProviderConfig{
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterFfiProviderConfig) Lower(value FfiProviderConfig) C.RustBuffer {
	return LowerIntoRustBuffer[FfiProviderConfig](c, value)
}

func (c FfiConverterFfiProviderConfig) Write(writer io.Writer, value FfiProviderConfig) {
	FfiConverterOptionalStringINSTANCE.Write(writer, value.AwsRegion)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.VaultAddress)
}

type FfiDestroyerFfiProviderConfig struct{}

func (_ FfiDestroyerFfiProviderConfig) Destroy(value FfiProviderConfig) {
	value.Destroy()
}

// Device attestation tier bound into the AAD for every operation.
type FfiAttestationTier uint

const (
	FfiAttestationTierSoftware       FfiAttestationTier = 1
	FfiAttestationTierHardwareBacked FfiAttestationTier = 2
	FfiAttestationTierConfidential   FfiAttestationTier = 3
)

type FfiConverterFfiAttestationTier struct{}

var FfiConverterFfiAttestationTierINSTANCE = FfiConverterFfiAttestationTier{}

func (c FfiConverterFfiAttestationTier) Lift(rb RustBufferI) FfiAttestationTier {
	return LiftFromRustBuffer[FfiAttestationTier](c, rb)
}

func (c FfiConverterFfiAttestationTier) Lower(value FfiAttestationTier) C.RustBuffer {
	return LowerIntoRustBuffer[FfiAttestationTier](c, value)
}
func (FfiConverterFfiAttestationTier) Read(reader io.Reader) FfiAttestationTier {
	id := readInt32(reader)
	return FfiAttestationTier(id)
}

func (FfiConverterFfiAttestationTier) Write(writer io.Writer, value FfiAttestationTier) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerFfiAttestationTier struct{}

func (_ FfiDestroyerFfiAttestationTier) Destroy(value FfiAttestationTier) {
}

// The min-custody-posture dial accepts `PlatformManaged` here (unlike
// `EncryptInput.posture` which only admits Byok/Hyok). A floor of
// `PlatformManaged` means "no floor — admit every posture including PM", which
// is the default / most permissive setting.
type FfiMinPosture uint

const (
	FfiMinPosturePlatformManaged FfiMinPosture = 1
	FfiMinPostureByok            FfiMinPosture = 2
	FfiMinPostureHyok            FfiMinPosture = 3
)

type FfiConverterFfiMinPosture struct{}

var FfiConverterFfiMinPostureINSTANCE = FfiConverterFfiMinPosture{}

func (c FfiConverterFfiMinPosture) Lift(rb RustBufferI) FfiMinPosture {
	return LiftFromRustBuffer[FfiMinPosture](c, rb)
}

func (c FfiConverterFfiMinPosture) Lower(value FfiMinPosture) C.RustBuffer {
	return LowerIntoRustBuffer[FfiMinPosture](c, value)
}
func (FfiConverterFfiMinPosture) Read(reader io.Reader) FfiMinPosture {
	id := readInt32(reader)
	return FfiMinPosture(id)
}

func (FfiConverterFfiMinPosture) Write(writer io.Writer, value FfiMinPosture) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerFfiMinPosture struct{}

func (_ FfiDestroyerFfiMinPosture) Destroy(value FfiMinPosture) {
}

// Oracle routing mode. `None` defaults to `Brokered`.
type FfiOracleMode uint

const (
	FfiOracleModeBrokered FfiOracleMode = 1
	FfiOracleModeDirect   FfiOracleMode = 2
)

type FfiConverterFfiOracleMode struct{}

var FfiConverterFfiOracleModeINSTANCE = FfiConverterFfiOracleMode{}

func (c FfiConverterFfiOracleMode) Lift(rb RustBufferI) FfiOracleMode {
	return LiftFromRustBuffer[FfiOracleMode](c, rb)
}

func (c FfiConverterFfiOracleMode) Lower(value FfiOracleMode) C.RustBuffer {
	return LowerIntoRustBuffer[FfiOracleMode](c, value)
}
func (FfiConverterFfiOracleMode) Read(reader io.Reader) FfiOracleMode {
	id := readInt32(reader)
	return FfiOracleMode(id)
}

func (FfiConverterFfiOracleMode) Write(writer io.Writer, value FfiOracleMode) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerFfiOracleMode struct{}

func (_ FfiDestroyerFfiOracleMode) Destroy(value FfiOracleMode) {
}

// Custody posture for a `zk_encrypt` call. `PlatformManaged` is deliberately
// absent — `kibo-kms-rest` is the KEK-free customer-KMS layer; a PM call has
// no customer KMS to talk to and is structurally impossible here.
type FfiPosture uint

const (
	FfiPostureByok FfiPosture = 1
	FfiPostureHyok FfiPosture = 2
)

type FfiConverterFfiPosture struct{}

var FfiConverterFfiPostureINSTANCE = FfiConverterFfiPosture{}

func (c FfiConverterFfiPosture) Lift(rb RustBufferI) FfiPosture {
	return LiftFromRustBuffer[FfiPosture](c, rb)
}

func (c FfiConverterFfiPosture) Lower(value FfiPosture) C.RustBuffer {
	return LowerIntoRustBuffer[FfiPosture](c, value)
}
func (FfiConverterFfiPosture) Read(reader io.Reader) FfiPosture {
	id := readInt32(reader)
	return FfiPosture(id)
}

func (FfiConverterFfiPosture) Write(writer io.Writer, value FfiPosture) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerFfiPosture struct{}

func (_ FfiDestroyerFfiPosture) Destroy(value FfiPosture) {
}

// Explicit strategy pin. `None` selects the posture default (`LocalEnvelope`).
type FfiStrategy uint

const (
	FfiStrategyLocalEnvelope     FfiStrategy = 1
	FfiStrategyCustomerKmsOracle FfiStrategy = 2
)

type FfiConverterFfiStrategy struct{}

var FfiConverterFfiStrategyINSTANCE = FfiConverterFfiStrategy{}

func (c FfiConverterFfiStrategy) Lift(rb RustBufferI) FfiStrategy {
	return LiftFromRustBuffer[FfiStrategy](c, rb)
}

func (c FfiConverterFfiStrategy) Lower(value FfiStrategy) C.RustBuffer {
	return LowerIntoRustBuffer[FfiStrategy](c, value)
}
func (FfiConverterFfiStrategy) Read(reader io.Reader) FfiStrategy {
	id := readInt32(reader)
	return FfiStrategy(id)
}

func (FfiConverterFfiStrategy) Write(writer io.Writer, value FfiStrategy) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerFfiStrategy struct{}

func (_ FfiDestroyerFfiStrategy) Destroy(value FfiStrategy) {
}

// The customer KMS provider this build targets. Because `kibo-kms-rest`
// compiles exactly one provider per build (mutually-exclusive Cargo features),
// this value is validated against the compiled-in provider at runtime and
// returns `K9O_CRYPTO_STRATEGY_UNAVAILABLE` if they do not match.
type KmsProvider uint

const (
	KmsProviderAwsKms         KmsProvider = 1
	KmsProviderGcpKms         KmsProvider = 2
	KmsProviderAzureKv        KmsProvider = 3
	KmsProviderHashicorpVault KmsProvider = 4
)

type FfiConverterKmsProvider struct{}

var FfiConverterKmsProviderINSTANCE = FfiConverterKmsProvider{}

func (c FfiConverterKmsProvider) Lift(rb RustBufferI) KmsProvider {
	return LiftFromRustBuffer[KmsProvider](c, rb)
}

func (c FfiConverterKmsProvider) Lower(value KmsProvider) C.RustBuffer {
	return LowerIntoRustBuffer[KmsProvider](c, value)
}
func (FfiConverterKmsProvider) Read(reader io.Reader) KmsProvider {
	id := readInt32(reader)
	return KmsProvider(id)
}

func (FfiConverterKmsProvider) Write(writer io.Writer, value KmsProvider) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerKmsProvider struct{}

func (_ FfiDestroyerKmsProvider) Destroy(value KmsProvider) {
}

// uniffi-facing error type for `zk_encrypt` / `zk_decrypt` native exports.
//
// `ZkFfiError` is a struct and cannot directly implement `uniffi::Error`
// (which requires an enum). This enum flattens the struct into three
// variants — platform error, upstream KMS error, and posture floor
// violation — that uniffi-generated Swift/Kotlin/Python code can pattern-match.
// Converted from `ZkFfiError` at the `ffi_uniffi` boundary.
type ZkFfiUniffiError struct {
	err error
}

// Convience method to turn *ZkFfiUniffiError into error
// Avoiding treating nil pointer as non nil error interface
func (err *ZkFfiUniffiError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err ZkFfiUniffiError) Error() string {
	return fmt.Sprintf("ZkFfiUniffiError: %s", err.err.Error())
}

func (err ZkFfiUniffiError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrZkFfiUniffiErrorPlatform = fmt.Errorf("ZkFfiUniffiErrorPlatform")
var ErrZkFfiUniffiErrorUpstreamKms = fmt.Errorf("ZkFfiUniffiErrorUpstreamKms")
var ErrZkFfiUniffiErrorPostureViolation = fmt.Errorf("ZkFfiUniffiErrorPostureViolation")

// Variant structs
// A platform error with a `K9O_*` wire code.
type ZkFfiUniffiErrorPlatform struct {
	Code    string
	Message string
}

// A platform error with a `K9O_*` wire code.
func NewZkFfiUniffiErrorPlatform(
	code string,
	message string,
) *ZkFfiUniffiError {
	return &ZkFfiUniffiError{err: &ZkFfiUniffiErrorPlatform{
		Code:    code,
		Message: message}}
}

func (e ZkFfiUniffiErrorPlatform) destroy() {
	FfiDestroyerString{}.Destroy(e.Code)
	FfiDestroyerString{}.Destroy(e.Message)
}

func (err ZkFfiUniffiErrorPlatform) Error() string {
	return fmt.Sprint("Platform",
		": ",

		"Code=",
		err.Code,
		", ",
		"Message=",
		err.Message,
	)
}

func (self ZkFfiUniffiErrorPlatform) Is(target error) bool {
	return target == ErrZkFfiUniffiErrorPlatform
}

// The customer's KMS returned an error. No platform code is assigned
// (upstream failures are not `K9O_*` codes).
type ZkFfiUniffiErrorUpstreamKms struct {
	Message string
}

// The customer's KMS returned an error. No platform code is assigned
// (upstream failures are not `K9O_*` codes).
func NewZkFfiUniffiErrorUpstreamKms(
	message string,
) *ZkFfiUniffiError {
	return &ZkFfiUniffiError{err: &ZkFfiUniffiErrorUpstreamKms{
		Message: message}}
}

func (e ZkFfiUniffiErrorUpstreamKms) destroy() {
	FfiDestroyerString{}.Destroy(e.Message)
}

func (err ZkFfiUniffiErrorUpstreamKms) Error() string {
	return fmt.Sprint("UpstreamKms",
		": ",

		"Message=",
		err.Message,
	)
}

func (self ZkFfiUniffiErrorUpstreamKms) Is(target error) bool {
	return target == ErrZkFfiUniffiErrorUpstreamKms
}

// The requested posture or oracle mode is below the project-declared
// policy floor (`K9O_CRYPTO_POSTURE_FLOOR_VIOLATION`).
type ZkFfiUniffiErrorPostureViolation struct {
	Code      string
	Message   string
	Dimension string
	Requested string
	Declared  string
}

// The requested posture or oracle mode is below the project-declared
// policy floor (`K9O_CRYPTO_POSTURE_FLOOR_VIOLATION`).
func NewZkFfiUniffiErrorPostureViolation(
	code string,
	message string,
	dimension string,
	requested string,
	declared string,
) *ZkFfiUniffiError {
	return &ZkFfiUniffiError{err: &ZkFfiUniffiErrorPostureViolation{
		Code:      code,
		Message:   message,
		Dimension: dimension,
		Requested: requested,
		Declared:  declared}}
}

func (e ZkFfiUniffiErrorPostureViolation) destroy() {
	FfiDestroyerString{}.Destroy(e.Code)
	FfiDestroyerString{}.Destroy(e.Message)
	FfiDestroyerString{}.Destroy(e.Dimension)
	FfiDestroyerString{}.Destroy(e.Requested)
	FfiDestroyerString{}.Destroy(e.Declared)
}

func (err ZkFfiUniffiErrorPostureViolation) Error() string {
	return fmt.Sprint("PostureViolation",
		": ",

		"Code=",
		err.Code,
		", ",
		"Message=",
		err.Message,
		", ",
		"Dimension=",
		err.Dimension,
		", ",
		"Requested=",
		err.Requested,
		", ",
		"Declared=",
		err.Declared,
	)
}

func (self ZkFfiUniffiErrorPostureViolation) Is(target error) bool {
	return target == ErrZkFfiUniffiErrorPostureViolation
}

type FfiConverterZkFfiUniffiError struct{}

var FfiConverterZkFfiUniffiErrorINSTANCE = FfiConverterZkFfiUniffiError{}

func (c FfiConverterZkFfiUniffiError) Lift(eb RustBufferI) *ZkFfiUniffiError {
	return LiftFromRustBuffer[*ZkFfiUniffiError](c, eb)
}

func (c FfiConverterZkFfiUniffiError) Lower(value *ZkFfiUniffiError) C.RustBuffer {
	return LowerIntoRustBuffer[*ZkFfiUniffiError](c, value)
}

func (c FfiConverterZkFfiUniffiError) Read(reader io.Reader) *ZkFfiUniffiError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &ZkFfiUniffiError{&ZkFfiUniffiErrorPlatform{
			Code:    FfiConverterStringINSTANCE.Read(reader),
			Message: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 2:
		return &ZkFfiUniffiError{&ZkFfiUniffiErrorUpstreamKms{
			Message: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 3:
		return &ZkFfiUniffiError{&ZkFfiUniffiErrorPostureViolation{
			Code:      FfiConverterStringINSTANCE.Read(reader),
			Message:   FfiConverterStringINSTANCE.Read(reader),
			Dimension: FfiConverterStringINSTANCE.Read(reader),
			Requested: FfiConverterStringINSTANCE.Read(reader),
			Declared:  FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterZkFfiUniffiError.Read()", errorID))
	}
}

func (c FfiConverterZkFfiUniffiError) Write(writer io.Writer, value *ZkFfiUniffiError) {
	switch variantValue := value.err.(type) {
	case *ZkFfiUniffiErrorPlatform:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Code)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
	case *ZkFfiUniffiErrorUpstreamKms:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
	case *ZkFfiUniffiErrorPostureViolation:
		writeInt32(writer, 3)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Code)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Dimension)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Requested)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Declared)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterZkFfiUniffiError.Write", value))
	}
}

type FfiDestroyerZkFfiUniffiError struct{}

func (_ FfiDestroyerZkFfiUniffiError) Destroy(value *ZkFfiUniffiError) {
	switch variantValue := value.err.(type) {
	case ZkFfiUniffiErrorPlatform:
		variantValue.destroy()
	case ZkFfiUniffiErrorUpstreamKms:
		variantValue.destroy()
	case ZkFfiUniffiErrorPostureViolation:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerZkFfiUniffiError.Destroy", value))
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

type FfiConverterOptionalFfiAllowedOracleModes struct{}

var FfiConverterOptionalFfiAllowedOracleModesINSTANCE = FfiConverterOptionalFfiAllowedOracleModes{}

func (c FfiConverterOptionalFfiAllowedOracleModes) Lift(rb RustBufferI) *FfiAllowedOracleModes {
	return LiftFromRustBuffer[*FfiAllowedOracleModes](c, rb)
}

func (_ FfiConverterOptionalFfiAllowedOracleModes) Read(reader io.Reader) *FfiAllowedOracleModes {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterFfiAllowedOracleModesINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalFfiAllowedOracleModes) Lower(value *FfiAllowedOracleModes) C.RustBuffer {
	return LowerIntoRustBuffer[*FfiAllowedOracleModes](c, value)
}

func (_ FfiConverterOptionalFfiAllowedOracleModes) Write(writer io.Writer, value *FfiAllowedOracleModes) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterFfiAllowedOracleModesINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalFfiAllowedOracleModes struct{}

func (_ FfiDestroyerOptionalFfiAllowedOracleModes) Destroy(value *FfiAllowedOracleModes) {
	if value != nil {
		FfiDestroyerFfiAllowedOracleModes{}.Destroy(*value)
	}
}

type FfiConverterOptionalFfiPosturePolicy struct{}

var FfiConverterOptionalFfiPosturePolicyINSTANCE = FfiConverterOptionalFfiPosturePolicy{}

func (c FfiConverterOptionalFfiPosturePolicy) Lift(rb RustBufferI) *FfiPosturePolicy {
	return LiftFromRustBuffer[*FfiPosturePolicy](c, rb)
}

func (_ FfiConverterOptionalFfiPosturePolicy) Read(reader io.Reader) *FfiPosturePolicy {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterFfiPosturePolicyINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalFfiPosturePolicy) Lower(value *FfiPosturePolicy) C.RustBuffer {
	return LowerIntoRustBuffer[*FfiPosturePolicy](c, value)
}

func (_ FfiConverterOptionalFfiPosturePolicy) Write(writer io.Writer, value *FfiPosturePolicy) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterFfiPosturePolicyINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalFfiPosturePolicy struct{}

func (_ FfiDestroyerOptionalFfiPosturePolicy) Destroy(value *FfiPosturePolicy) {
	if value != nil {
		FfiDestroyerFfiPosturePolicy{}.Destroy(*value)
	}
}

type FfiConverterOptionalFfiMinPosture struct{}

var FfiConverterOptionalFfiMinPostureINSTANCE = FfiConverterOptionalFfiMinPosture{}

func (c FfiConverterOptionalFfiMinPosture) Lift(rb RustBufferI) *FfiMinPosture {
	return LiftFromRustBuffer[*FfiMinPosture](c, rb)
}

func (_ FfiConverterOptionalFfiMinPosture) Read(reader io.Reader) *FfiMinPosture {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterFfiMinPostureINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalFfiMinPosture) Lower(value *FfiMinPosture) C.RustBuffer {
	return LowerIntoRustBuffer[*FfiMinPosture](c, value)
}

func (_ FfiConverterOptionalFfiMinPosture) Write(writer io.Writer, value *FfiMinPosture) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterFfiMinPostureINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalFfiMinPosture struct{}

func (_ FfiDestroyerOptionalFfiMinPosture) Destroy(value *FfiMinPosture) {
	if value != nil {
		FfiDestroyerFfiMinPosture{}.Destroy(*value)
	}
}

type FfiConverterOptionalFfiOracleMode struct{}

var FfiConverterOptionalFfiOracleModeINSTANCE = FfiConverterOptionalFfiOracleMode{}

func (c FfiConverterOptionalFfiOracleMode) Lift(rb RustBufferI) *FfiOracleMode {
	return LiftFromRustBuffer[*FfiOracleMode](c, rb)
}

func (_ FfiConverterOptionalFfiOracleMode) Read(reader io.Reader) *FfiOracleMode {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterFfiOracleModeINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalFfiOracleMode) Lower(value *FfiOracleMode) C.RustBuffer {
	return LowerIntoRustBuffer[*FfiOracleMode](c, value)
}

func (_ FfiConverterOptionalFfiOracleMode) Write(writer io.Writer, value *FfiOracleMode) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterFfiOracleModeINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalFfiOracleMode struct{}

func (_ FfiDestroyerOptionalFfiOracleMode) Destroy(value *FfiOracleMode) {
	if value != nil {
		FfiDestroyerFfiOracleMode{}.Destroy(*value)
	}
}

type FfiConverterOptionalFfiStrategy struct{}

var FfiConverterOptionalFfiStrategyINSTANCE = FfiConverterOptionalFfiStrategy{}

func (c FfiConverterOptionalFfiStrategy) Lift(rb RustBufferI) *FfiStrategy {
	return LiftFromRustBuffer[*FfiStrategy](c, rb)
}

func (_ FfiConverterOptionalFfiStrategy) Read(reader io.Reader) *FfiStrategy {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterFfiStrategyINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalFfiStrategy) Lower(value *FfiStrategy) C.RustBuffer {
	return LowerIntoRustBuffer[*FfiStrategy](c, value)
}

func (_ FfiConverterOptionalFfiStrategy) Write(writer io.Writer, value *FfiStrategy) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterFfiStrategyINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalFfiStrategy struct{}

func (_ FfiDestroyerOptionalFfiStrategy) Destroy(value *FfiStrategy) {
	if value != nil {
		FfiDestroyerFfiStrategy{}.Destroy(*value)
	}
}

type FfiConverterSequenceFfiKvPair struct{}

var FfiConverterSequenceFfiKvPairINSTANCE = FfiConverterSequenceFfiKvPair{}

func (c FfiConverterSequenceFfiKvPair) Lift(rb RustBufferI) []FfiKvPair {
	return LiftFromRustBuffer[[]FfiKvPair](c, rb)
}

func (c FfiConverterSequenceFfiKvPair) Read(reader io.Reader) []FfiKvPair {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]FfiKvPair, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterFfiKvPairINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceFfiKvPair) Lower(value []FfiKvPair) C.RustBuffer {
	return LowerIntoRustBuffer[[]FfiKvPair](c, value)
}

func (c FfiConverterSequenceFfiKvPair) Write(writer io.Writer, value []FfiKvPair) {
	if len(value) > math.MaxInt32 {
		panic("[]FfiKvPair is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterFfiKvPairINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceFfiKvPair struct{}

func (FfiDestroyerSequenceFfiKvPair) Destroy(sequence []FfiKvPair) {
	for _, value := range sequence {
		FfiDestroyerFfiKvPair{}.Destroy(value)
	}
}

const (
	uniffiRustFuturePollReady      int8 = 0
	uniffiRustFuturePollMaybeReady int8 = 1
)

type rustFuturePollFunc func(C.uint64_t, C.UniffiRustFutureContinuationCallback, C.uint64_t)
type rustFutureCompleteFunc[T any] func(C.uint64_t, *C.RustCallStatus) T
type rustFutureFreeFunc func(C.uint64_t)

//export kibo_kms_rest_ffi_uniffiFutureContinuationCallback
func kibo_kms_rest_ffi_uniffiFutureContinuationCallback(data C.uint64_t, pollResult C.int8_t) {
	h := cgo.Handle(uintptr(data))
	waiter := h.Value().(chan int8)
	waiter <- int8(pollResult)
}

func uniffiRustCallAsync[E any, T any, F any](
	errConverter BufReader[*E],
	completeFunc rustFutureCompleteFunc[F],
	liftFunc func(F) T,
	rustFuture C.uint64_t,
	pollFunc rustFuturePollFunc,
	freeFunc rustFutureFreeFunc,
) (T, *E) {
	defer freeFunc(rustFuture)

	pollResult := int8(-1)
	waiter := make(chan int8, 1)

	chanHandle := cgo.NewHandle(waiter)
	defer chanHandle.Delete()

	for pollResult != uniffiRustFuturePollReady {
		pollFunc(
			rustFuture,
			(C.UniffiRustFutureContinuationCallback)(C.kibo_kms_rest_ffi_uniffiFutureContinuationCallback),
			C.uint64_t(chanHandle),
		)
		pollResult = <-waiter
	}

	var goValue T
	var ffiValue F
	var err *E

	ffiValue, err = rustCallWithError(errConverter, func(status *C.RustCallStatus) F {
		return completeFunc(rustFuture, status)
	})
	if err != nil {
		return goValue, err
	}
	return liftFunc(ffiValue), nil
}

//export kibo_kms_rest_ffi_uniffiFreeGorutine
func kibo_kms_rest_ffi_uniffiFreeGorutine(data C.uint64_t) {
	handle := cgo.Handle(uintptr(data))
	defer handle.Delete()

	guard := handle.Value().(chan struct{})
	guard <- struct{}{}
}

// uniffi-exported async decrypt. Returns the recovered plaintext bytes.
//
// # Errors
// [`ZkFfiUniffiError`] on a posture-floor violation, an upstream customer-KMS
// failure, or a platform error.
func ZkDecrypt(input DecryptInput) (DecryptOutput, error) {
	res, err := uniffiRustCallAsync[ZkFfiUniffiError](
		FfiConverterZkFfiUniffiErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_kibo_kms_rest_ffi_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) DecryptOutput {
			return FfiConverterDecryptOutputINSTANCE.Lift(ffi)
		},
		C.uniffi_kibo_kms_rest_ffi_fn_func_zk_decrypt(FfiConverterDecryptInputINSTANCE.Lower(input)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_kibo_kms_rest_ffi_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_kibo_kms_rest_ffi_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

// Sync (blocking) decrypt — drives the canonical async `zk_decrypt`.
//
// # Errors
// [`ZkFfiUniffiError`] on a posture-floor violation, an upstream customer-KMS
// failure, or a platform error.
func ZkDecryptBlocking(input DecryptInput) (DecryptOutput, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[ZkFfiUniffiError](FfiConverterZkFfiUniffiError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_kibo_kms_rest_ffi_fn_func_zk_decrypt_blocking(FfiConverterDecryptInputINSTANCE.Lower(input), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue DecryptOutput
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterDecryptOutputINSTANCE.Lift(_uniffiRV), nil
	}
}

// uniffi-exported async encrypt. Returns the canonical `b"KIBO"` envelope bytes.
//
// # Errors
// [`ZkFfiUniffiError`] on a posture-floor violation, an upstream customer-KMS
// failure, or a platform error.
func ZkEncrypt(input EncryptInput) (EncryptOutput, error) {
	res, err := uniffiRustCallAsync[ZkFfiUniffiError](
		FfiConverterZkFfiUniffiErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_kibo_kms_rest_ffi_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) EncryptOutput {
			return FfiConverterEncryptOutputINSTANCE.Lift(ffi)
		},
		C.uniffi_kibo_kms_rest_ffi_fn_func_zk_encrypt(FfiConverterEncryptInputINSTANCE.Lower(input)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_kibo_kms_rest_ffi_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_kibo_kms_rest_ffi_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

// Sync (blocking) encrypt — drives the canonical async `zk_encrypt` to
// completion. For the non-async-native uniffi languages; crypto-identical.
//
// # Errors
// [`ZkFfiUniffiError`] on a posture-floor violation, an upstream customer-KMS
// failure, or a platform error.
func ZkEncryptBlocking(input EncryptInput) (EncryptOutput, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[ZkFfiUniffiError](FfiConverterZkFfiUniffiError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_kibo_kms_rest_ffi_fn_func_zk_encrypt_blocking(FfiConverterEncryptInputINSTANCE.Lower(input), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue EncryptOutput
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterEncryptOutputINSTANCE.Lift(_uniffiRV), nil
	}
}
