package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

const (
	encryptedBackupFormat        = "rclone-manager-encrypted-backup"
	encryptedBackupVersion       = 1
	backupKDFName                = "pbkdf2-sha256"
	backupCipherName             = "aes-256-gcm"
	defaultBackupPBKDF2Iterations = 600000
	minBackupPBKDF2Iterations    = 10000
	maxBackupPBKDF2Iterations    = 5000000
	backupSaltBytes              = 16
	backupKeyBytes               = 32
	minBackupPassphraseLen       = 8
)

// EncryptedBackupEnvelope stores encrypted full-backup payloads using PBKDF2-HMAC-SHA256 and AES-256-GCM.
type EncryptedBackupEnvelope struct {
	Format     string `json:"format"`
	Version    int    `json:"version"`
	KDF        string `json:"kdf"`
	Iterations int    `json:"iterations"`
	Cipher     string `json:"cipher"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type fullBackupPayload struct {
	Version    int                   `json:"version"`
	ExportedAt time.Time             `json:"exported_at"`
	Settings   domain.GlobalSettings `json:"settings"`
	Jobs       []domain.Job          `json:"jobs"`
	RcloneConf string                `json:"rclone_conf"`
}

// FullImportResult summarizes restored items from a full encrypted backup restore.
type FullImportResult struct {
	JobsImported    int      `json:"jobs_imported"`
	RemotesImported int      `json:"remotes_imported"`
	RemoteNames     []string `json:"remote_names"`
}

// EncryptBackupPayload encrypts plaintext bytes with the supplied passphrase using PBKDF2-SHA256 and AES-256-GCM.
func EncryptBackupPayload(plaintext []byte, passphrase string, iterations int) ([]byte, error) {
	if len(passphrase) < minBackupPassphraseLen {
		return nil, fmt.Errorf("%w: backup passphrase must be at least %d characters", ErrValidation, minBackupPassphraseLen)
	}
	if iterations < minBackupPBKDF2Iterations || iterations > maxBackupPBKDF2Iterations {
		return nil, fmt.Errorf("%w: invalid PBKDF2 iteration count %d", ErrValidation, iterations)
	}

	salt := make([]byte, backupSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate backup salt: %w", err)
	}

	key, err := pbkdf2.Key(sha256.New, passphrase, salt, iterations, backupKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("derive backup encryption key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM cipher: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate backup nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	env := EncryptedBackupEnvelope{
		Format:     encryptedBackupFormat,
		Version:    encryptedBackupVersion,
		KDF:        backupKDFName,
		Iterations: iterations,
		Cipher:     backupCipherName,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
	return json.MarshalIndent(env, "", "  ")
}

// DecryptBackupPayload validates and decrypts an EncryptedBackupEnvelope using the supplied passphrase.
func DecryptBackupPayload(raw []byte, passphrase string) ([]byte, error) {
	if passphrase == "" {
		return nil, fmt.Errorf("%w: backup passphrase is required", ErrValidation)
	}

	var env EncryptedBackupEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("%w: invalid backup file format: %v", ErrValidation, err)
	}
	if env.Format != encryptedBackupFormat || env.Version != encryptedBackupVersion {
		return nil, fmt.Errorf("%w: unsupported backup format or version", ErrValidation)
	}
	if env.KDF != backupKDFName || env.Cipher != backupCipherName {
		return nil, fmt.Errorf("%w: unsupported backup encryption parameters", ErrValidation)
	}
	if env.Iterations < minBackupPBKDF2Iterations || env.Iterations > maxBackupPBKDF2Iterations {
		return nil, fmt.Errorf("%w: invalid backup key derivation iterations", ErrValidation)
	}

	salt, err := base64.StdEncoding.DecodeString(env.Salt)
	if err != nil || len(salt) < 8 {
		return nil, fmt.Errorf("%w: invalid backup salt", ErrValidation)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid backup nonce", ErrValidation)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil || len(ciphertext) == 0 {
		return nil, fmt.Errorf("%w: invalid backup ciphertext", ErrValidation)
	}

	key, err := pbkdf2.Key(sha256.New, passphrase, salt, env.Iterations, backupKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("derive backup decryption key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM cipher: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("%w: invalid backup nonce length", ErrValidation)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid backup passphrase or corrupted backup file", ErrValidation)
	}
	return plaintext, nil
}

// ExportEncryptedBackup exports raw rclone.conf, global settings, and jobs into an encrypted .rcmbak envelope.
func (s *AppService) ExportEncryptedBackup(ctx context.Context, passphrase string) ([]byte, error) {
	if len(passphrase) < minBackupPassphraseLen {
		return nil, fmt.Errorf("%w: backup passphrase must be at least %d characters", ErrValidation, minBackupPassphraseLen)
	}

	bundle, err := s.ExportBundle(ctx)
	if err != nil {
		return nil, err
	}

	var rawConf string
	if s.browser != nil {
		rawConf, err = s.browser.ReadRawConf(ctx)
		if err != nil {
			return nil, fmt.Errorf("read rclone.conf: %w", err)
		}
	}

	payload := fullBackupPayload{
		Version:    encryptedBackupVersion,
		ExportedAt: bundle.ExportedAt,
		Settings:   bundle.Settings,
		Jobs:       bundle.Jobs,
		RcloneConf: rawConf,
	}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return EncryptBackupPayload(plaintext, passphrase, defaultBackupPBKDF2Iterations)
}

// ImportEncryptedBackup decrypts, validates, and restores rclone.conf, global settings, and jobs.
func (s *AppService) ImportEncryptedBackup(ctx context.Context, raw []byte, passphrase, mode string) (FullImportResult, error) {
	if mode != "merge" && mode != "replace" {
		return FullImportResult{}, fmt.Errorf("%w: mode must be merge or replace", ErrValidation)
	}
	if passphrase == "" {
		return FullImportResult{}, fmt.Errorf("%w: backup passphrase is required", ErrValidation)
	}

	plaintext, err := DecryptBackupPayload(raw, passphrase)
	if err != nil {
		return FullImportResult{}, err
	}

	var payload fullBackupPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return FullImportResult{}, fmt.Errorf("%w: corrupted backup payload: %v", ErrValidation, err)
	}
	if payload.Version != encryptedBackupVersion {
		return FullImportResult{}, fmt.Errorf("%w: unsupported backup payload version %d", ErrValidation, payload.Version)
	}

	// Validate settings and jobs before mutating rclone.conf or DB.
	if err := validateSettingsFields(&payload.Settings); err != nil {
		return FullImportResult{}, err
	}
	for i := range payload.Jobs {
		if err := ValidateJob(&payload.Jobs[i]); err != nil {
			return FullImportResult{}, fmt.Errorf("%w: job %q: %v", ErrValidation, payload.Jobs[i].Name, err)
		}
	}

	importedRemotes := []string{}
	if strings.TrimSpace(payload.RcloneConf) != "" {
		if s.browser == nil {
			return FullImportResult{}, errors.New("rclone client unavailable")
		}
		names, err := s.browser.ImportConf(ctx, payload.RcloneConf, mode)
		if err != nil {
			return FullImportResult{}, fmt.Errorf("%w: import rclone.conf: %v", ErrValidation, err)
		}
		if names != nil {
			importedRemotes = names
		}
	}

	if err := s.ImportBundle(ctx, domain.ExportBundle{
		Version:    payload.Version,
		ExportedAt: payload.ExportedAt,
		Settings:   payload.Settings,
		Jobs:       payload.Jobs,
	}, mode); err != nil {
		return FullImportResult{}, err
	}

	return FullImportResult{
		JobsImported:    len(payload.Jobs),
		RemotesImported: len(importedRemotes),
		RemoteNames:     importedRemotes,
	}, nil
}
