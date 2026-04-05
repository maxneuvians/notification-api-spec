package signing

import (
	"errors"
	"strings"
)

const apiKeySalt = "signer_api_key"

func SignAPIKeyToken(token, secret string) (string, error) {
	if strings.TrimSpace(token) == "" {
		return "", errors.New("token is required")
	}

	if strings.TrimSpace(secret) == "" {
		return "", errors.New("secret is required")
	}

	return base64Encode(hmacSignature(deriveKey(secret, apiKeySalt), []byte(token))), nil
}

func SignAPIKeyTokenWithAllKeys(token string, secrets []string) ([]string, error) {
	seen := make(map[string]struct{}, len(secrets))
	variants := make([]string, 0, len(secrets))

	for _, rawSecret := range secrets {
		secret := strings.TrimSpace(rawSecret)
		if secret == "" {
			continue
		}

		variant, err := SignAPIKeyToken(token, secret)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[variant]; ok {
			continue
		}

		seen[variant] = struct{}{}
		variants = append(variants, variant)
	}

	if len(variants) == 0 {
		return nil, errors.New("at least one secret is required")
	}

	return variants, nil
}
