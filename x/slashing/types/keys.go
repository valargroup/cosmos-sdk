package types

import (
	"encoding/binary"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
	"github.com/cosmos/cosmos-sdk/types/kv"
)

const (
	// ModuleName is the name of the module
	ModuleName = "slashing"

	// StoreKey is the store key string for slashing
	StoreKey = ModuleName

	// RouterKey is the message route for slashing
	RouterKey = ModuleName

	// MissedBlockBitmapChunkSize defines the chunk size, in number of bits, of a
	// validator missed block bitmap. Chunks are used to reduce the storage and
	// write overhead of IAVL nodes. The total size of the bitmap is roughly in
	// the range [0, SignedBlocksWindow) where each bit represents a block. A
	// validator's IndexOffset modulo the SignedBlocksWindow is used to retrieve
	// the chunk in that bitmap range. Once the chunk is retrieved, the same index
	// is used to check or flip a bit, where if a bit is set, it indicates the
	// validator missed that block.
	//
	// For a bitmap of N items, i.e. a validator's signed block window, the amount
	// of write complexity per write with a factor of f being the overhead of
	// IAVL being un-optimized, i.e. 2-4, is as follows:
	//
	// ChunkSize + (f * 256 <IAVL leaf hash>) + 256 * log_2(N / ChunkSize)
	//
	// As for the storage overhead, with the same factor f, it is as follows:
	// (N - 256) + (N / ChunkSize) * (512 * f)
	MissedBlockBitmapChunkSize = 1024 // 2^10 bits
)

// Keys for slashing store
// Items are stored with the following key: values
//
// - 0x01<consAddrLen (1 Byte)><consAddress_Bytes>: ValidatorSigningInfo
//
// - 0x02<consAddrLen (1 Byte)><consAddress_Bytes><chunk_index>: bitmap_chunk
//
// - 0x03<accAddrLen (1 Byte)><accAddr_Bytes>: cryptotypes.PubKey
//
// - 0x04<consAddrLen (1 Byte)><consAddress_Bytes><height>: sparse missed block
//
// - 0x05<height><consAddrLen (1 Byte)><consAddress_Bytes>: sparse missed block pruning index

var (
	ParamsKey                           = []byte{0x00} // Prefix for params key
	ValidatorSigningInfoKeyPrefix       = []byte{0x01} // Prefix for signing info
	ValidatorMissedBlockBitmapKeyPrefix = []byte{0x02} // Prefix for missed block bitmap
	AddrPubkeyRelationKeyPrefix         = []byte{0x03} // Prefix for address-pubkey relation
	ValidatorMissedBlockHeightKeyPrefix = []byte{0x04} // Prefix for sparse missed block height markers
	MissedBlockPruningIndexKeyPrefix    = []byte{0x05} // Prefix for sparse missed block pruning index
)

// ValidatorSigningInfoKey - stored by *Consensus* address (not operator address)
func ValidatorSigningInfoKey(v sdk.ConsAddress) []byte {
	return append(ValidatorSigningInfoKeyPrefix, address.MustLengthPrefix(v.Bytes())...)
}

// ValidatorSigningInfoAddress - extract the address from a validator signing info key
func ValidatorSigningInfoAddress(key []byte) (v sdk.ConsAddress) {
	// Remove prefix and address length.
	kv.AssertKeyAtLeastLength(key, 3)
	addr := key[2:]

	return sdk.ConsAddress(addr)
}

// ValidatorMissedBlockBitmapPrefixKey returns the key prefix for a validator's
// missed block bitmap.
func ValidatorMissedBlockBitmapPrefixKey(v sdk.ConsAddress) []byte {
	return append(ValidatorMissedBlockBitmapKeyPrefix, address.MustLengthPrefix(v.Bytes())...)
}

// ValidatorMissedBlockBitmapKey returns the key for a validator's missed block
// bitmap chunk.
func ValidatorMissedBlockBitmapKey(v sdk.ConsAddress, chunkIndex int64) []byte {
	bz := make([]byte, 8)
	binary.LittleEndian.PutUint64(bz, uint64(chunkIndex))

	return append(ValidatorMissedBlockBitmapPrefixKey(v), bz...)
}

// ValidatorMissedBlockHeightPrefixKey returns the prefix for sparse missed block
// markers for a validator.
func ValidatorMissedBlockHeightPrefixKey(v sdk.ConsAddress) []byte {
	return append(ValidatorMissedBlockHeightKeyPrefix, address.MustLengthPrefix(v.Bytes())...)
}

// ValidatorMissedBlockHeightKey returns the sparse missed block marker key for a
// validator at a concrete block height.
func ValidatorMissedBlockHeightKey(v sdk.ConsAddress, height int64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(height))

	return append(ValidatorMissedBlockHeightPrefixKey(v), bz...)
}

// MissedBlockPruningIndexKey returns an ordered index key for pruning sparse
// missed block markers older than the active signed block window.
func MissedBlockPruningIndexKey(height int64, v sdk.ConsAddress) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(height))

	key := append(MissedBlockPruningIndexKeyPrefix, bz...)
	return append(key, address.MustLengthPrefix(v.Bytes())...)
}

// AddrPubkeyRelationKey gets pubkey relation key used to get the pubkey from the address
func AddrPubkeyRelationKey(addr []byte) []byte {
	return append(AddrPubkeyRelationKeyPrefix, address.MustLengthPrefix(addr)...)
}
