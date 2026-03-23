package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/SpellingDragon/wechat-robot-go/wechat"
)

func main() {
	// Create bot with default options
	bot := wechat.NewBot(
		wechat.WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))),
	)

	// Setup context with signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Login (scan QR code on first run, reuses token afterwards)
	fmt.Println("Logging in...")
	err := bot.Login(ctx, func(qrCodeImgContent string) {
		fmt.Println("Please scan the QR code with WeChat:")
		fmt.Println(qrCodeImgContent)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Login successful!")

	// Register message handler
	bot.OnMessage(func(ctx context.Context, msg *wechat.Message) error {
		slog.Info("received message",
			"from", msg.FromUserID,
			"is_image", msg.IsImage(),
			"is_voice", msg.IsVoice(),
			"is_file", msg.IsFile(),
			"is_video", msg.IsVideo(),
			"text", msg.Text(),
		)

		// CDN base URL for media download
		cdnBaseURL := bot.CDNBaseURL()

		// Handle image messages - echo back the image
		if msg.IsImage() {
			img := msg.GetImageItem()
			if img != nil && img.Media != nil {
				slog.Info("processing image message",
					"has_aeskey", img.AESKey != "",
					"media_aeskey", img.Media.AESKey,
				)

				// Download the image
				imageData, err := bot.DownloadImage(ctx, msg, cdnBaseURL)
				if err != nil {
					slog.Error("failed to download image", "error", err)
					// Still try to echo back even if download fails
				} else {
					slog.Info("downloaded image", "size", len(imageData))

					// Save image to temp file
					tmpDir := os.TempDir()
					tmpFile := filepath.Join(tmpDir, fmt.Sprintf("echo_%d.png", time.Now().UnixNano()))
					if err := os.WriteFile(tmpFile, imageData, 0644); err != nil {
						slog.Error("failed to save image", "error", err)
					} else {
						slog.Info("saved image to", "path", tmpFile)

						// Echo back the image
						if err := bot.SendImageFromPath(ctx, msg.FromUserID, tmpFile); err != nil {
							slog.Error("failed to send image echo", "error", err)
						} else {
							slog.Info("sent image echo", "to", msg.FromUserID)
						}

						// Clean up
						os.Remove(tmpFile)
					}
				}
			}
			return nil
		}

		// Handle voice messages - echo back the voice
		if msg.IsVoice() {
			voice := msg.GetVoiceItem()
			if voice != nil && voice.Media != nil {
				slog.Info("processing voice message")

				// Download the voice
				voiceData, err := bot.DownloadVoice(ctx, voice, cdnBaseURL)
				if err != nil {
					slog.Error("failed to download voice", "error", err)
				} else {
					slog.Info("downloaded voice", "size", len(voiceData))

					// Save voice to temp file
					tmpDir := os.TempDir()
					tmpFile := filepath.Join(tmpDir, fmt.Sprintf("echo_%d.silk", time.Now().UnixNano()))
					if err := os.WriteFile(tmpFile, voiceData, 0644); err != nil {
						slog.Error("failed to save voice", "error", err)
					} else {
						slog.Info("saved voice to", "path", tmpFile)

						// Echo back the voice
						if err := bot.SendVoiceFromPath(ctx, msg.FromUserID, tmpFile, voice.Duration); err != nil {
							slog.Error("failed to send voice echo", "error", err)
						} else {
							slog.Info("sent voice echo", "to", msg.FromUserID)
						}

						// Clean up
						os.Remove(tmpFile)
					}
				}
			}
			return nil
		}

		// Handle file messages
		if msg.IsFile() {
			file := msg.GetFileItem()
			if file != nil && file.Media != nil {
				slog.Info("processing file message", "filename", file.FileName)

				// Download the file
				fileData, err := bot.DownloadFileFromItem(ctx, file, cdnBaseURL)
				if err != nil {
					slog.Error("failed to download file", "error", err)
				} else {
					slog.Info("downloaded file", "size", len(fileData), "filename", file.FileName)

					// Save file to temp
					tmpDir := os.TempDir()
					tmpFile := filepath.Join(tmpDir, fmt.Sprintf("echo_%s", file.FileName))
					if err := os.WriteFile(tmpFile, fileData, 0644); err != nil {
						slog.Error("failed to save file", "error", err)
					} else {
						slog.Info("saved file to", "path", tmpFile)

						// Echo back the file
						if err := bot.SendFileFromPath(ctx, msg.FromUserID, tmpFile); err != nil {
							slog.Error("failed to send file echo", "error", err)
						} else {
							slog.Info("sent file echo", "to", msg.FromUserID)
						}

						// Clean up
						os.Remove(tmpFile)
					}
				}
			}
			return nil
		}

		// Handle video messages
		if msg.IsVideo() {
			video := msg.GetVideoItem()
			if video != nil && video.Media != nil {
				slog.Info("processing video message")

				// Download the video
				videoData, err := bot.DownloadVideoFromItem(ctx, video, cdnBaseURL)
				if err != nil {
					slog.Error("failed to download video", "error", err)
				} else {
					slog.Info("downloaded video", "size", len(videoData))

					// Save video to temp file
					tmpDir := os.TempDir()
					tmpFile := filepath.Join(tmpDir, fmt.Sprintf("echo_%d.mp4", time.Now().UnixNano()))
					if err := os.WriteFile(tmpFile, videoData, 0644); err != nil {
						slog.Error("failed to save video", "error", err)
					} else {
						slog.Info("saved video to", "path", tmpFile)

						// Echo back the video
						if err := bot.SendVideoFromPath(ctx, msg.FromUserID, tmpFile); err != nil {
							slog.Error("failed to send video echo", "error", err)
						} else {
							slog.Info("sent video echo", "to", msg.FromUserID)
						}

						// Clean up
						os.Remove(tmpFile)
					}
				}
			}
			return nil
		}

		// Handle text messages
		text := msg.Text()
		if text == "" {
			return nil
		}

		// Show typing indicator
		_ = bot.SendTyping(ctx, msg.FromUserID)

		// Reply with echo
		reply := fmt.Sprintf("Echo: %s", text)
		if err := bot.Reply(ctx, msg, reply); err != nil {
			slog.Error("failed to reply", "error", err)
			return err
		}

		// Stop typing
		_ = bot.StopTyping(ctx, msg.FromUserID)

		slog.Info("sent reply", "to", msg.FromUserID, "text", reply)
		return nil
	})

	// Run the bot
	fmt.Println("Bot is running. Press Ctrl+C to stop.")
	if err := bot.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Bot stopped with error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Bot stopped gracefully.")
}
