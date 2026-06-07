// Package backup builds and restores complete Sempa data bundles (database +
// attachment blobs) as a single portable file, optionally encrypted.
package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/scrypt"
)

// Encrypted-bundle wire format:
//
//	magic[8] = "SEMPABK1"
//	salt[16]            scrypt salt
//	noncePrefix[8]      random; per-chunk nonce = noncePrefix || counter(uint32)
//	then repeated frames until a frame with the final flag set:
//	  flag[1]           1 = last frame
//	  len[4]            big-endian length of the ciphertext that follows
//	  ciphertext[len]   AES-256-GCM(plaintext chunk), AAD = counter(4) || flag(1)
//
// Chunking keeps memory bounded (GCM is not streaming) and the counter in the
// nonce + AAD binds chunk order, while the final flag prevents truncation.
const (
	magic          = "SEMPABK1"
	saltLen        = 16
	noncePrefixLen = 8
	chunkSize      = 64 * 1024

	scryptN = 1 << 15
	scryptR = 8
	scryptP = 1
	keyLen  = 32
)

func deriveKey(passphrase string, salt []byte) ([]byte, error) {
	return scrypt.Key([]byte(passphrase), salt, scryptN, scryptR, scryptP, keyLen)
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// EncryptStream encrypts everything read from src and writes the bundle to dst.
func EncryptStream(dst io.Writer, src io.Reader, passphrase string) error {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	noncePrefix := make([]byte, noncePrefixLen)
	if _, err := rand.Read(noncePrefix); err != nil {
		return err
	}
	key, err := deriveKey(passphrase, salt)
	if err != nil {
		return err
	}
	aead, err := newAEAD(key)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(dst, magic); err != nil {
		return err
	}
	if _, err := dst.Write(salt); err != nil {
		return err
	}
	if _, err := dst.Write(noncePrefix); err != nil {
		return err
	}

	buf := make([]byte, chunkSize)
	nonce := make([]byte, aead.NonceSize())
	copy(nonce, noncePrefix)
	var counter uint32
	for {
		n, readErr := io.ReadFull(src, buf)
		if readErr == io.EOF {
			n = 0
		}
		final := readErr == io.EOF || readErr == io.ErrUnexpectedEOF
		if readErr != nil && !final {
			return readErr
		}

		flag := byte(0)
		if final {
			flag = 1
		}
		binary.BigEndian.PutUint32(nonce[noncePrefixLen:], counter)
		aad := []byte{0, 0, 0, 0, flag}
		binary.BigEndian.PutUint32(aad, counter)

		ct := aead.Seal(nil, nonce, buf[:n], aad)

		hdr := make([]byte, 5)
		hdr[0] = flag
		binary.BigEndian.PutUint32(hdr[1:], uint32(len(ct)))
		if _, err := dst.Write(hdr); err != nil {
			return err
		}
		if _, err := dst.Write(ct); err != nil {
			return err
		}

		counter++
		if final {
			return nil
		}
	}
}

// DecryptStream reverses EncryptStream.
func DecryptStream(dst io.Writer, src io.Reader, passphrase string) error {
	hdr := make([]byte, len(magic)+saltLen+noncePrefixLen)
	if _, err := io.ReadFull(src, hdr); err != nil {
		return fmt.Errorf("read header: %w", err)
	}
	if string(hdr[:len(magic)]) != magic {
		return errors.New("not a Sempa encrypted backup")
	}
	salt := hdr[len(magic) : len(magic)+saltLen]
	noncePrefix := hdr[len(magic)+saltLen:]

	key, err := deriveKey(passphrase, salt)
	if err != nil {
		return err
	}
	aead, err := newAEAD(key)
	if err != nil {
		return err
	}

	nonce := make([]byte, aead.NonceSize())
	copy(nonce, noncePrefix)
	var counter uint32
	lenHdr := make([]byte, 5)
	for {
		if _, err := io.ReadFull(src, lenHdr); err != nil {
			return fmt.Errorf("read frame header: %w", err)
		}
		flag := lenHdr[0]
		ctLen := binary.BigEndian.Uint32(lenHdr[1:])
		ct := make([]byte, ctLen)
		if _, err := io.ReadFull(src, ct); err != nil {
			return fmt.Errorf("read frame: %w", err)
		}

		binary.BigEndian.PutUint32(nonce[noncePrefixLen:], counter)
		aad := []byte{0, 0, 0, 0, flag}
		binary.BigEndian.PutUint32(aad, counter)

		pt, err := aead.Open(nil, nonce, ct, aad)
		if err != nil {
			return errors.New("decryption failed — wrong passphrase or corrupt backup")
		}
		if _, err := dst.Write(pt); err != nil {
			return err
		}

		counter++
		if flag == 1 {
			return nil
		}
	}
}

// LooksEncrypted reports whether the first bytes match the encrypted magic.
func LooksEncrypted(head []byte) bool {
	return len(head) >= len(magic) && string(head[:len(magic)]) == magic
}
