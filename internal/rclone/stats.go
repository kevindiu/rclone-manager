package rclone

import (
	"encoding/json"
	"strings"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

type jsonLogLine struct {
	Time   string        `json:"time"`
	Level  string        `json:"level"`
	Msg    string        `json:"msg"`
	Object string        `json:"object"`
	Size   int64         `json:"size"`
	Stats  *jsonLogStats `json:"stats"`
}

type jsonLogStats struct {
	Bytes        int64                 `json:"bytes"`
	TotalBytes   int64                 `json:"totalBytes"`
	Speed        float64               `json:"speed"`
	ETA          *float64              `json:"eta"`
	Transfers    int64                 `json:"transfers"`
	TotalTransf  int64                 `json:"totalTransfers"`
	Checks       int64                 `json:"checks"`
	TotalChecks  int64                 `json:"totalChecks"`
	Deletes      int64                 `json:"deletes"`
	Errors       int64                 `json:"errors"`
	ElapsedTime  float64               `json:"elapsedTime"`
	Transferring []jsonLogTransferring `json:"transferring"`
}

type jsonLogTransferring struct {
	Name       string   `json:"name"`
	Size       int64    `json:"size"`
	Bytes      int64    `json:"bytes"`
	Percentage int      `json:"percentage"`
	Speed      float64  `json:"speed"`
	SpeedAvg   float64  `json:"speedAvg"`
	ETA        *float64 `json:"eta"`
}

// ParseProgressLine attempts to parse an rclone --use-json-log line for live stats.
// Returns (progress, true) if the line contained stats, or (zero, false) otherwise.
func ParseProgressLine(line string) (domain.TransferProgress, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed[0] != '{' {
		return domain.TransferProgress{}, false
	}

	var entry jsonLogLine
	if err := json.Unmarshal([]byte(trimmed), &entry); err != nil || entry.Stats == nil {
		return domain.TransferProgress{}, false
	}

	st := entry.Stats
	var pct float64
	if st.TotalBytes > 0 {
		pct = (float64(st.Bytes) / float64(st.TotalBytes)) * 100
		if pct > 100 {
			pct = 100
		}
	} else if st.TotalTransf > 0 {
		pct = (float64(st.Transfers) / float64(st.TotalTransf)) * 100
		if pct > 100 {
			pct = 100
		}
	}

	var eta *int64
	if st.ETA != nil && *st.ETA >= 0 {
		v := int64(*st.ETA)
		eta = &v
	}

	transferring := make([]domain.TransferringFile, 0, len(st.Transferring))
	for _, tf := range st.Transferring {
		var fEta *int64
		if tf.ETA != nil && *tf.ETA >= 0 {
			v := int64(*tf.ETA)
			fEta = &v
		}
		transferring = append(transferring, domain.TransferringFile{
			Name:       tf.Name,
			Size:       tf.Size,
			Bytes:      tf.Bytes,
			Percentage: tf.Percentage,
			Speed:      tf.Speed,
			SpeedAvg:   tf.SpeedAvg,
			ETA:        fEta,
		})
	}

	return domain.TransferProgress{
		Bytes:          st.Bytes,
		TotalBytes:     st.TotalBytes,
		Percentage:     pct,
		Speed:          st.Speed,
		ETA:            eta,
		Transfers:      st.Transfers,
		TotalTransfers: st.TotalTransf,
		Checks:         st.Checks,
		TotalChecks:    st.TotalChecks,
		Deletes:        st.Deletes,
		Errors:         st.Errors,
		ElapsedTime:    st.ElapsedTime,
		Transferring:   transferring,
	}, true
}
