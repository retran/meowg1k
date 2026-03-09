package starlark

import (
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // MD5 is exposed as a user utility, not used for security
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewCryptoModule creates the crypto module.
func NewCryptoModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "crypto",
		Members: starlark.StringDict{
			"sha256": starlark.NewBuiltin("crypto.sha256", cryptoSha256),
			"md5":    starlark.NewBuiltin("crypto.md5", cryptoMd5),
			"hmac":   starlark.NewBuiltin("crypto.hmac", cryptoHmac),
		},
	}
}

// cryptoSha256 computes SHA256 hash.
func cryptoSha256(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data string
	if err := starlark.UnpackPositionalArgs("crypto.sha256", args, kwargs, 1, &data); err != nil {
		return nil, fmt.Errorf("crypto.sha256: %w", err)
	}

	hash := sha256.Sum256([]byte(data))
	return starlark.String(hex.EncodeToString(hash[:])), nil
}

// cryptoMd5 computes MD5 hash.
func cryptoMd5(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data string
	if err := starlark.UnpackPositionalArgs("crypto.md5", args, kwargs, 1, &data); err != nil {
		return nil, fmt.Errorf("crypto.md5: %w", err)
	}

	hash := md5.Sum([]byte(data)) //nolint:gosec // MD5 exposed as user utility
	return starlark.String(hex.EncodeToString(hash[:])), nil
}

// cryptoHmac computes HMAC-SHA256.
func cryptoHmac(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key, data string
	if err := starlark.UnpackPositionalArgs("crypto.hmac", args, kwargs, 2, &key, &data); err != nil {
		return nil, fmt.Errorf("crypto.hmac: %w", err)
	}

	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return starlark.String(hex.EncodeToString(h.Sum(nil))), nil
}
