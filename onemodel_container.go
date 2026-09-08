package oneocr

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

const maxModelBytes = 2 * 1024 * 1024 * 1024
const modelMarker uint64 = 0x252b081a4a

var modelKey = []byte("kj)TGtrK>f]b[Piow.gU+nC@s" + strings.Repeat("\"", 6) + "4")
var modelIV = []byte("Copyright @ OneO")

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) take(n uint64) ([]byte, error) {
	if n > uint64(len(r.data)-r.pos) {
		return nil, fmt.Errorf("truncated OneModel field at %d", r.pos)
	}
	b := r.data[r.pos : r.pos+int(n)]
	r.pos += int(n)
	return b, nil
}
func (r *byteReader) u64() (uint64, error) {
	b, e := r.take(8)
	if e != nil {
		return 0, e
	}
	return binary.LittleEndian.Uint64(b), nil
}
func (r *byteReader) blob(limit uint64) ([]byte, error) {
	n, e := r.u64()
	if e != nil {
		return nil, e
	}
	if n > limit {
		return nil, fmt.Errorf("OneModel field exceeds size limit")
	}
	return r.take(n)
}
func (r *byteReader) finish() error {
	if r.pos != len(r.data) {
		return fmt.Errorf("unexpected trailing OneModel bytes")
	}
	return nil
}

func decryptRecord(data, password []byte, salted bool) ([]byte, error) {
	r := byteReader{data: data}
	var salt, clear []byte
	var e error
	if salted {
		salt, e = r.take(16)
		if e != nil {
			return nil, e
		}
	}
	material := password
	if len(material) == 0 {
		clear, e = r.take(16)
		if e != nil {
			return nil, e
		}
		material = clear
	}
	encrypted := data[r.pos:]
	if len(encrypted) == 0 || len(encrypted)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid AES-CBC block length")
	}
	hash := sha256.New()
	hash.Write(material)
	hash.Write(salt)
	block, e := aes.NewCipher(hash.Sum(nil))
	if e != nil {
		return nil, e
	}
	plain := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, modelIV).CryptBlocks(plain, encrypted)
	padding := int(plain[len(plain)-1])
	if padding == 0 || padding > 16 || padding > len(plain) {
		return nil, fmt.Errorf("invalid OneModel padding or key")
	}
	for _, b := range plain[len(plain)-padding:] {
		if int(b) != padding {
			return nil, fmt.Errorf("invalid OneModel padding")
		}
	}
	plain = plain[:len(plain)-padding]
	var size, total, marker uint64
	if len(clear) > 0 {
		size = binary.LittleEndian.Uint64(clear)
		total = binary.LittleEndian.Uint64(clear[8:])
		r = byteReader{data: plain}
		marker, e = r.u64()
	} else {
		r = byteReader{data: plain}
		size, e = r.u64()
		if e != nil {
			return nil, e
		}
		total, e = r.u64()
		if e != nil {
			return nil, e
		}
		marker, e = r.u64()
	}
	if e != nil {
		return nil, e
	}
	if marker != modelMarker || size > maxModelBytes || total != size+24 {
		return nil, fmt.Errorf("unsupported OneModel marker or inconsistent lengths")
	}
	payload, e := r.take(size)
	if e != nil {
		return nil, e
	}
	return payload, r.finish()
}

type modelResource struct {
	name string
	data []byte
}
type decodedModel struct {
	hash      string
	config    []byte
	resources []modelResource
}

func decodeModel(data []byte) (*decodedModel, error) {
	if len(data) > maxModelBytes {
		return nil, fmt.Errorf("OneModel exceeds 2 GiB limit")
	}
	r := byteReader{data: data}
	encrypted, e := r.blob(maxModelBytes)
	if e != nil {
		return nil, e
	}
	index, e := decryptRecord(encrypted, modelKey, true)
	if e != nil {
		return nil, e
	}
	body, e := r.blob(maxModelBytes)
	if e != nil {
		return nil, e
	}
	if e = r.finish(); e != nil {
		return nil, e
	}
	r = byteReader{data: index}
	encrypted, e = r.blob(maxModelBytes)
	if e != nil {
		return nil, e
	}
	config, e := decryptRecord(encrypted, nil, true)
	if e != nil {
		return nil, e
	}
	count, e := r.u64()
	if e != nil || count == 0 || count > 4096 {
		return nil, fmt.Errorf("invalid OneModel resource count")
	}
	out := &decodedModel{hash: fmt.Sprintf("%x", sha256.Sum256(data)), config: config}
	names := map[string]bool{}
	var next uint64
	for i := uint64(0); i < count; i++ {
		encrypted, e = r.blob(16384)
		if e != nil {
			return nil, e
		}
		nameBytes, e := decryptRecord(encrypted, nil, false)
		if e != nil {
			return nil, e
		}
		name := string(nameBytes)
		if !utf8.Valid(nameBytes) || name == "" || strings.ContainsRune(name, 0) || names[name] {
			return nil, fmt.Errorf("invalid or duplicate OneModel resource name")
		}
		names[name] = true
		offset, e := r.u64()
		if e != nil {
			return nil, e
		}
		size, e := r.u64()
		if e != nil {
			return nil, e
		}
		flag, e := r.take(1)
		if e != nil {
			return nil, e
		}
		if flag[0] != 1 || offset != next || offset > uint64(len(body)) || size > uint64(len(body))-offset {
			return nil, fmt.Errorf("resource %d has invalid encoding or bounds", i)
		}
		payload, e := decryptRecord(body[int(offset):int(offset+size)], nil, true)
		if e != nil {
			return nil, fmt.Errorf("resource %d: %w", i, e)
		}
		out.resources = append(out.resources, modelResource{name, payload})
		next = offset + size
	}
	if next != uint64(len(body)) {
		return nil, fmt.Errorf("OneModel index does not cover body")
	}
	if e = r.finish(); e != nil {
		return nil, e
	}
	return out, nil
}

func readModel(filename string) (*decodedModel, error) {
	f, e := os.Open(filename)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	stat, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if stat.Size() > maxModelBytes {
		return nil, fmt.Errorf("model exceeds 2 GiB limit")
	}
	data, e := readLimited(f, maxModelBytes)
	if e != nil {
		return nil, e
	}
	return decodeModel(data)
}
