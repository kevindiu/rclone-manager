package api

// SystemInfo summarizes bundled rclone and server runtime details.
type SystemInfo struct {
	AppVersion    string `json:"app_version"`
	RcloneVersion string `json:"rclone_version"`
	RcloneOSArch  string `json:"rclone_os_arch,omitempty"`
	GoVersion     string `json:"go_version"`
	DataDir       string `json:"data_dir"`
	RcloneConfig  string `json:"rclone_config"`
	Timezone      string `json:"timezone"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}
