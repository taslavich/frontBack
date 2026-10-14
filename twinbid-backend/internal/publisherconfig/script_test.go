package publisherconfig

import "testing"

func TestPublicScriptURLOverrideAndLegacy(t *testing.T) {
	c := Config{PublicURL: "https://legacy.example.test/"}
	if c.ScriptURL() != "https://legacy.example.test/static/adManager.js" || c.ValidateScriptURL() != nil {
		t.Fatal(c.ScriptURL())
	}
	c.PublicScriptURL = "https://stable.example.test/sdk.js"
	if c.ScriptURL() != c.PublicScriptURL || c.ValidateScriptURL() != nil {
		t.Fatal(c.ScriptURL())
	}
	for _, raw := range []string{"http://stable.test/sdk.js", "https://user@stable.test/sdk.js", "https://stable.test:8443/sdk.js", "https://stable.test/sdk.js?url=evil", "https://stable.test/sdk.js#x", "https://stable.test", "https://127.0.0.1/sdk.js", "https://stable.test//sdk.js", "https://stable.test/../sdk.js"} {
		c.PublicScriptURL = raw
		if c.ValidateScriptURL() == nil {
			t.Fatal("accepted", raw)
		}
	}
}
