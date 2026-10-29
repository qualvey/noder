package contract

import "testing"

func TestXHTTPRequiresVlessTLSAndSNI(t *testing.T) {
	valid := map[string]interface{}{
		"tag": "xhttp", "server_address": "example.com", "server_port": 443,
		"security": "tls", "sni": "proxy.example.com", "transport_type": "xhttp",
	}
	if err := ValidateNodeContract(valid, "vless", true); err != nil {
		t.Fatalf("valid xhttp node rejected: %v", err)
	}

	for name, values := range map[string]map[string]interface{}{
		"reality": func() map[string]interface{} {
			copy := cloneValues(valid)
			copy["security"] = "reality"
			return copy
		}(),
		"missing sni": func() map[string]interface{} {
			copy := cloneValues(valid)
			delete(copy, "sni")
			return copy
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateNodeContract(values, "vless", true); err == nil {
				t.Fatal("expected xhttp validation error")
			}
		})
	}
}

func cloneValues(values map[string]interface{}) map[string]interface{} {
	copy := make(map[string]interface{}, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}
