// C-ABI shim over zao/ooz, exported from a DLL so Go can call it through
// syscall without cgo — the same approach internal/mpq uses for StormLib.
//
// The call sequences here mirror palooz (deafdudecomputers/PalworldSaveTools),
// which is the implementation known to produce saves Palworld accepts.
//
// ooz is GPL-3.0 (per powzix/ooz, which zao/ooz forks), so this whole project
// is GPL-3.0. See LICENSE and docs/THIRD_PARTY.md.

#include <cstdint>
#include <cstddef>

// From kraken.cpp.
int Kraken_Decompress(const uint8_t *src, size_t src_len, uint8_t *dst, size_t dst_len);

// From compress.cpp. palooz passes null for all three trailing pointers and
// lets ooz pick its own defaults; matching that keeps output byte-compatible.
struct CompressOptions;
struct LRMCascade;
int CompressBlock(int codec_id, uint8_t *src_in, uint8_t *dst_in, int src_size, int level,
                  const CompressOptions *compressopts, uint8_t *src_window_base, LRMCascade *lrm);

extern "C" {

// Codec ids, from compress.h.
enum {
    NanachiCodecKraken    = 8,
    NanachiCodecMermaid   = 9,
    NanachiCodecSelkie    = 11,
    NanachiCodecLeviathan = 13,
};

// dst must have room for dst_len + NANACHI_DECOMPRESS_PADDING bytes.
// Returns the number of bytes written, which the caller should check equals
// dst_len. Negative means failure.
__declspec(dllexport)
int64_t NanachiOozDecompress(const uint8_t *src, int64_t src_len,
                             uint8_t *dst, int64_t dst_len) {
    if (!src || !dst || src_len <= 0 || dst_len <= 0) {
        return -1;
    }
    return Kraken_Decompress(src, (size_t)src_len, dst, (size_t)dst_len);
}

// dst must have room for src_len + NANACHI_COMPRESS_PADDING bytes.
// Returns the compressed length, or negative on failure.
__declspec(dllexport)
int64_t NanachiOozCompress(int32_t codec_id, int32_t level,
                           const uint8_t *src, int64_t src_len,
                           uint8_t *dst, int64_t dst_cap) {
    if (!src || !dst || src_len <= 0 || dst_cap <= 0) {
        return -1;
    }
    int rc = CompressBlock(codec_id, const_cast<uint8_t *>(src), dst,
                           (int)src_len, level, nullptr, nullptr, nullptr);
    if (rc < 0 || (int64_t)rc > dst_cap) {
        return -2;
    }
    return rc;
}

} // extern "C"
