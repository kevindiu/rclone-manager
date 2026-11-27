package rclone

import "testing"

func TestParseProgressLine(t *testing.T) {
	// Non-JSON / non-stats lines return false.
	for _, input := range []string{"", "plain log line", "{bad json", `{"level":"info","msg":"started"}`} {
		if _, ok := ParseProgressLine(input); ok {
			t.Fatalf("expected ok=false for %q", input)
		}
	}

	// Normal stats line with transferring files and ETA.
	line := `{"stats":{"bytes":500,"totalBytes":1000,"speed":250.5,"eta":2.0,"transfers":1,"totalTransfers":2,"checks":3,"totalChecks":4,"deletes":1,"errors":0,"elapsedTime":4.5,"transferring":[{"name":"a/file.bin","size":1000,"bytes":500,"percentage":50,"speed":250,"speedAvg":200,"eta":2.0},{"name":"b.bin","size":100,"bytes":0,"percentage":0,"speed":0,"speedAvg":0,"eta":-1}]}}`
	prog, ok := ParseProgressLine(line)
	if !ok {
		t.Fatal("expected ok=true for valid stats line")
	}
	if prog.Percentage != 50 || prog.ETA == nil || *prog.ETA != 2 {
		t.Fatalf("unexpected progress summary: %+v", prog)
	}
	if prog.Transfers != 1 || prog.TotalTransfers != 2 || prog.Deletes != 1 || prog.Checks != 3 {
		t.Fatalf("unexpected transfer counters: %+v", prog)
	}
	if len(prog.Transferring) != 2 || prog.Transferring[0].ETA == nil || *prog.Transferring[0].ETA != 2 || prog.Transferring[1].ETA != nil {
		t.Fatalf("unexpected transferring entries: %+v", prog.Transferring)
	}

	// totalBytes == 0 falls back to transfer count percentage and clamps > 100.
	fallbackLine := `{"stats":{"bytes":0,"totalBytes":0,"transfers":3,"totalTransfers":2,"eta":-5}}`
	prog2, ok := ParseProgressLine(fallbackLine)
	if !ok {
		t.Fatal("expected ok=true for fallback stats line")
	}
	if prog2.Percentage != 100 || prog2.ETA != nil {
		t.Fatalf("expected percentage clamped to 100 and nil ETA, got %+v", prog2)
	}

	// Bytes > totalBytes clamps to 100.
	clampLine := `{"stats":{"bytes":2048,"totalBytes":1024}}`
	prog3, ok := ParseProgressLine(clampLine)
	if !ok || prog3.Percentage != 100 {
		t.Fatalf("expected bytes percentage clamped to 100, got ok=%v prog=%+v", ok, prog3)
	}
}
