//go:build rugaroo_prelude

package prelude

import (
	"crypto"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // OpenSSL::HMAC with "MD5" is what the program asked for
	"crypto/pbkdf2"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // OpenSSL::HMAC with "SHA1" is what the program asked for
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/pem"
	"hash"
	"strings"
)

// rbRSAParse accepts every PEM key form OpenSSL::PKey::RSA.new does: PKIX/PKCS #1 public, PKCS #1/#8 private.
func rbRSAParse(s string) *OpenSSL_PKey_RSA {
	fail := func() *OpenSSL_PKey_RSA {
		panic(NewOpenSSL_PKey_PKeyError(Ref(String("Neither PUB key nor PRIV key"))))
	}
	block, _ := pem.Decode([]byte(s))
	if block == nil {
		return fail()
	}
	if k, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if pub, ok := k.(*rsa.PublicKey); ok {
			return &OpenSSL_PKey_RSA{pub: pub}
		}
		return fail()
	}
	if pub, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return &OpenSSL_PKey_RSA{pub: pub}
	}
	if priv, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return &OpenSSL_PKey_RSA{pub: &priv.PublicKey, priv: priv}
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if priv, ok := k.(*rsa.PrivateKey); ok {
			return &OpenSSL_PKey_RSA{pub: &priv.PublicKey, priv: priv}
		}
	}
	return fail()
}

// rbRSAHashName normalises an OpenSSL digest name ("SHA512", "sha-512") to a key of rbRSAHash.
func rbRSAHashName(name string) string {
	return strings.ReplaceAll(strings.ToUpper(name), "-", "")
}

// rbRSAHash is the crypto.Hash an OpenSSL digest name names and data's sum under it.
func rbRSAHash(name, data string) (crypto.Hash, []byte) {
	var h crypto.Hash
	switch rbRSAHashName(name) {
	case "SHA1":
		h = crypto.SHA1
	case "SHA224":
		h = crypto.SHA224
	case "SHA256":
		h = crypto.SHA256
	case "SHA384":
		h = crypto.SHA384
	case "SHA512":
		h = crypto.SHA512
	default:
		panic(NewOpenSSL_PKey_PKeyError(Ref(String("unsupported digest algorithm (" + name + ")"))))
	}
	w := h.New()
	var buf [32 << 10]byte // in pieces: []byte(data) would allocate and copy a multi-megabyte input whole
	for len(data) > 0 {
		n := copy(buf[:], data)
		w.Write(buf[:n])
		data = data[n:]
	}
	return h, w.Sum(nil)
}

// rbRSASaltLength maps verify_pss's salt_length: to Go's PSSOptions.
func rbRSASaltLength(v any, h crypto.Hash) int {
	switch x := v.(type) {
	case Symbol:
		if x == "digest" {
			return h.Size()
		}
		return rsa.PSSSaltLengthAuto // :auto and :max both accept any salt when verifying
	case Integer:
		return int(x)
	}
	return rsa.PSSSaltLengthAuto
}

// rbOpenSSLDigest is the hash an OpenSSL digest name ("SHA256", "sha-1") names, as HMAC and PBKDF2 take it.
func rbOpenSSLDigest(name string) func() hash.Hash {
	switch rbRSAHashName(name) {
	case "MD5":
		return md5.New
	case "SHA1":
		return sha1.New
	case "SHA224":
		return sha256.New224
	case "SHA256":
		return sha256.New
	case "SHA384":
		return sha512.New384
	case "SHA512":
		return sha512.New
	}
	panic(NewRuntimeError(Ref(String("Unsupported digest algorithm (" + name + ").: unknown object name"))))
}

// rbHMAC is OpenSSL::HMAC.digest: the raw MAC of data under key.
func rbHMAC(digest, key, data string) string {
	m := hmac.New(rbOpenSSLDigest(digest), []byte(key))
	m.Write([]byte(data))
	return string(m.Sum(nil))
}

// rbPBKDF2 is OpenSSL::PKCS5.pbkdf2_hmac.
func rbPBKDF2(pass, salt string, iter, keylen int, digest string) string {
	k, err := pbkdf2.Key(rbOpenSSLDigest(digest), pass, []byte(salt), iter, keylen)
	if err != nil {
		panic(NewOpenSSL_OpenSSLError(Ref(String(err.Error()))))
	}
	return string(k)
}
