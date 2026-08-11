package lang

import (
	"os"
	"testing"
)

func parseOne(t *testing.T, src string) *Program {
	t.Helper()
	prog, errs := ParseFile(src, "test.scute")
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	return prog
}

func findSpawn(t *testing.T, prog *Program) *SpawnBlock {
	t.Helper()
	for _, s := range prog.Statements {
		if b, ok := s.(*SpawnBlock); ok {
			return b
		}
	}
	t.Fatal("no spawn block found")
	return nil
}

func fieldValue(fields []*Field, key string) (Node, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f.Value, true
		}
	}
	return nil, false
}

// #79: keyword-shaped attribute keys like "export:" must parse inside blocks.
func TestParse_KeywordFieldKeys_InSpawn(t *testing.T) {
	prog := parseOne(t, `
spawn "proxmox:storage" as "gecko-nfs"
  storage: "gecko-nfs"
  type:    "nfs"
  server:  "192.168.1.10"
  export:  "/mnt/storage"
  from:    "somewhere"
  content: "images,rootdir,iso"
end
`)
	b := findSpawn(t, prog)
	for _, key := range []string{"storage", "type", "server", "export", "from", "content"} {
		if _, ok := fieldValue(b.Fields, key); !ok {
			t.Errorf("field %q missing from spawn block: got %d fields", key, len(b.Fields))
		}
	}
}

func TestParse_KeywordFieldKeys_InHabitat(t *testing.T) {
	prog := parseOne(t, `
habitat "proxmox"
  endpoint: "https://pve:8006"
  export:   "unusual-but-legal"
end
`)
	for _, s := range prog.Statements {
		if h, ok := s.(*HabitatBlock); ok {
			if _, found := fieldValue(h.Fields, "export"); !found {
				t.Errorf("export field missing from habitat: %+v", h.Fields)
			}
			return
		}
	}
	t.Fatal("no habitat block found")
}

func TestParse_KeywordWithoutColon_StillErrors(t *testing.T) {
	_, errs := ParseFile(`
spawn "proxmox:vm" as "x"
  export "not-a-field"
end
`, "test.scute")
	if len(errs) == 0 {
		t.Fatal("bare keyword without ':' inside spawn should still be an error")
	}
}

func TestParse_EndStillTerminatesBlocks(t *testing.T) {
	prog := parseOne(t, `
spawn "proxmox:vm" as "x"
  memory: 2048
end

spawn "proxmox:vm" as "y"
  cores: 2
end
`)
	count := 0
	for _, s := range prog.Statements {
		if _, ok := s.(*SpawnBlock); ok {
			count++
		}
	}
	if count != 2 {
		t.Errorf("want 2 spawn blocks, got %d", count)
	}
}

// The example project that originally surfaced #79 must parse cleanly.
func TestParse_ProxmoxExample(t *testing.T) {
	src, err := os.ReadFile("../../examples/proxmox/main.scute")
	if err != nil {
		t.Skipf("example file not available: %v", err)
	}
	_, errs := ParseFile(string(src), "main.scute")
	if len(errs) > 0 {
		t.Fatalf("examples/proxmox/main.scute should parse: %v", errs)
	}
}
