package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Intelblox/Multiblox/internal/app"
	"github.com/Intelblox/Multiblox/internal/httputil"
	"github.com/hugolgst/rich-go/client"
	"golang.org/x/sys/windows/registry"
)

type LogMessage struct {
	DateTime    time.Time `json:"dateTime"`
	ElapsedTime float64   `json:"elapsedTime"`
	ThreadID    string    `json:"threadId"`
	LogLevel    string    `json:"logLevel"`
	Category    string    `json:"category"`
	Content     string    `json:"message"`
}

func ParseLogMessage(line string) (*LogMessage, error) {
	end := strings.Index(line, ",")
	if end == -1 {
		return nil, fmt.Errorf("failed to find terminator for datetime")
	}
	dateTime, err := time.Parse("2006-01-02T15:04:05.000Z", line[:end])
	if err != nil {
		return nil, fmt.Errorf("failed to parse datetime: %s", err)
	}
	line = line[end+1:]
	end = strings.Index(line, ",")
	if end == -1 {
		return nil, fmt.Errorf("failed to find terminator for elapsed time")
	}
	elapsedTime, err := strconv.ParseFloat(line[:end], 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse elapsed time: %s", err)
	}
	line = line[end+1:]
	end = strings.Index(line, ",")
	if end == -1 {
		return nil, fmt.Errorf("failed to find terminator for thread ID")
	}
	threadId := line[:end]
	line = line[end+1:]
	end = strings.Index(line, " ")
	if end == -1 {
		return nil, fmt.Errorf("failed to find terminator for log level")
	}
	logLevel := line[:end]
	line = line[end+1:]
	end = strings.Index(line, " ")
	if end == -1 {
		return nil, fmt.Errorf("failed to find terminator for category")
	}
	category := strings.Trim(line[:end], "[]")
	line = line[end+1:]
	content := line
	logMsg := &LogMessage{
		DateTime:    dateTime,
		ElapsedTime: elapsedTime,
		ThreadID:    threadId,
		LogLevel:    logLevel,
		Category:    category,
		Content:     content,
	}
	return logMsg, nil
}

var ServerType uint8

func OnJoinGame(msg *LogMessage) error {
	ServerType = 0
	return nil
}

func OnJoinGamePrivate(msg *LogMessage) error {
	ServerType = 1
	return nil
}

func OnJoinGameReserved(msg *LogMessage) error {
	ServerType = 2
	return nil
}

func OnGameLoad(msg *LogMessage) error {
	err := client.Login("1252892459197792297")
	if err != nil {
		return fmt.Errorf("failed to shake hands with discord: %s", err)
	}
	activity := client.Activity{
		SmallImage: "roblox",
		SmallText:  "Roblox",
	}
	line := msg.Content[strings.Index(msg.Content, ":")+2:]
	keyValues := strings.Split(line, ", ")
	var wg sync.WaitGroup
	var serverId string
	var placeId string
	for _, keyValue := range keyValues {
		keyValueSlice := strings.Split(keyValue, ":")
		if len(keyValueSlice) != 2 {
			continue
		}
		key := keyValueSlice[0]
		value := keyValueSlice[1]
		if key == "universeid" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var games map[string]any
				url := fmt.Sprintf("https://games.roblox.com/v1/games?universeIds=%s", value)
				err = httputil.GetJson(url, &games)
				if err != nil {
					err = fmt.Errorf("failed to fetch JSON response: %s", err)
					return
				}
				data, ok := games["data"].([]any)
				if !ok {
					err = fmt.Errorf("failed to find or assert game data slice")
					return
				}
				if len(data) == 0 {
					err = fmt.Errorf("failed to find game data inside slice")
					return
				}
				game, ok := data[0].(map[string]any)
				if !ok {
					err = fmt.Errorf("failed to assert type of game data to map")
					return
				}
				title, ok := game["name"].(string)
				if !ok {
					err = fmt.Errorf("failed to find or assert game title")
					return
				}
				creator, ok := game["creator"].(map[string]any)
				if !ok {
					err = fmt.Errorf("failed to find or assert game creator")
					return
				}
				creatorName, ok := creator["name"].(string)
				if !ok {
					err = fmt.Errorf("failed to find or assert creator name")
					return
				}
				activity.Details = fmt.Sprintf("Playing %s", title)
				activity.LargeText = title
				if ServerType == 0 {
					hasVerifiedBadge, ok := creator["hasVerifiedBadge"].(bool)
					if !ok {
						err = fmt.Errorf("failed to find or assert verified badge")
						return
					}
					if hasVerifiedBadge {
						creatorName += " ☑️"
					}
					activity.State = fmt.Sprintf("By @%s", creatorName)
				}
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				var icons map[string]any
				url := fmt.Sprintf("https://thumbnails.roblox.com/v1/games/icons?universeIds=%s&returnPolicy=PlaceHolder&size=512x512&format=Png&isCircular=false", value)
				err = httputil.GetJson(url, &icons)
				if err != nil {
					err = fmt.Errorf("failed to fetch JSON response: %s", err)
					return
				}
				data, ok := icons["data"].([]any)
				if !ok {
					err = fmt.Errorf("failed to find or assert icon data slice")
					return
				}
				if len(data) == 0 {
					err = fmt.Errorf("failed to find icon data inside slice")
					return
				}
				icon, ok := data[0].(map[string]any)
				if !ok {
					err = fmt.Errorf("failed to assert type of icon to map")
					return
				}
				imageUrl, ok := icon["imageUrl"].(string)
				if !ok {
					err = fmt.Errorf("failed to find or assert image URL")
					return
				}
				activity.LargeImage = imageUrl
			}()
		} else if key == "clienttime" {
			var clientTimeInt float64
			clientTimeInt, err = strconv.ParseFloat(value, 64)
			if err != nil {
				err = fmt.Errorf("error parsing client time: %s", err)
				continue
			}
			clientTime := time.Unix(int64(clientTimeInt), 0)
			activity.Timestamps = &client.Timestamps{
				Start: &clientTime,
			}
		} else if key == "sid" {
			serverId = value
		} else if key == "placeid" {
			placeId = value
		}
	}
	wg.Wait()
	if err != nil {
		return err
	}
	appKey, err := registry.OpenKey(registry.CURRENT_USER, app.ConfigKey, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to open app key: %s", err)
	}
	defer appKey.Close()
	discordShowJoinServer, _, err := appKey.GetIntegerValue("DiscordShowJoinServer")
	if err != nil {
		return fmt.Errorf("failed to get DiscordShowJoinServer value: %s", err)
	}
	if ServerType == 0 && discordShowJoinServer == 1 {
		activity.Buttons = append(activity.Buttons, &client.Button{
			Label: "Join Server",
			Url:   fmt.Sprintf("roblox://experiences/start?placeId=%s&gameInstanceId=%s", placeId, serverId),
		})
	} else if ServerType == 1 {
		activity.State = "In a private server"
	} else if ServerType == 2 {
		activity.State = "In a reserved server"
	}
	activity.Buttons = append(activity.Buttons, &client.Button{
		Label: "See Game Page",
		Url:   fmt.Sprintf("https://www.roblox.com/games/%s", placeId),
	})
	err = client.SetActivity(activity)
	if err != nil {
		return fmt.Errorf("failed to set discord activity: %s", err)
	}
	return nil
}

func OnBloxstrapRPC() {

}

func OnGameWillClose(msg *LogMessage) error {
	client.Logout()
	return nil
}

func DiscordRPC() error {
	homeDir, _ := os.UserHomeDir()
	rbxLogsDir := filepath.Join(homeDir, "AppData", "Local", "Roblox", "Logs")
	var logFileName string
	var logFileCreationTime int64
	startTimestamp := time.Now().Unix()
	for {
		entries, err := os.ReadDir(rbxLogsDir)
		if err != nil {
			return fmt.Errorf("failed to read roblox logs directory: %s", err)
		}
		for _, entry := range entries {
			entryInfo, err := entry.Info()
			if err != nil {
				return fmt.Errorf("failed to check entry info: %s", err)
			}
			entryInfoWin32, ok := entryInfo.Sys().(*syscall.Win32FileAttributeData)
			if !ok {
				return fmt.Errorf("failed to assert entry info to win32 type")
			}
			entryName := entry.Name()
			entryCreationTime := entryInfoWin32.CreationTime.Nanoseconds()
			if entryInfo.IsDir() ||
				!strings.HasSuffix(entryName, ".log") ||
				time.Now().UnixNano()-entryCreationTime > (15*time.Second).Nanoseconds() ||
				entryCreationTime < logFileCreationTime {
				continue
			}
			logFileName = filepath.Join(rbxLogsDir, entryName)
			logFileCreationTime = entryCreationTime
		}
		if logFileName != "" {
			break
		}
		if time.Now().Unix()-startTimestamp > 15 {
			return fmt.Errorf("failed to find a log file to read from within 15 seconds")
		}
		time.Sleep(time.Second)
	}
	stat, err := os.Stat(logFileName)
	if err != nil {
		return fmt.Errorf("failed to fetch log file info: %s", err)
	}
	lastModTime := stat.ModTime()
	var lastSize int64
	for {
		logFile, err := os.Open(logFileName)
		if err != nil {
			return fmt.Errorf("failed to open log file: %s", err)
		}
		stat, err := logFile.Stat()
		if err != nil {
			return fmt.Errorf("failed to fetch log file info: %s", err)
		}
		currentSize := stat.Size()
		if currentSize-lastSize < 0 {
			return fmt.Errorf("log file has been tampered with and no longer readable")
		}
		section := io.NewSectionReader(logFile, lastSize, currentSize)
		scanner := bufio.NewScanner(section)
		events := [][]any{
			{"FLog::GameJoinUtil", "GameJoinUtil::joinGame", OnJoinGame},
			{"FLog::GameJoinUtil", "GameJoinUtil::joinGamePostPrivateServer", OnJoinGamePrivate},
			{"FLog::GameJoinUtil", "GameJoinUtil::initiateTeleportToReservedServer", OnJoinGameReserved},
			{"FLog::GameJoinLoadTime", "Report game_join_loadtime", OnGameLoad},
			{"FLog::SingleSurfaceApp", "handleGameWillClose", OnGameWillClose},
		}
		for scanner.Scan() {
			text := scanner.Text()
			logMsg, err := ParseLogMessage(text)
			if err != nil {
				continue
			}
			for _, event := range events {
				if len(event) < 3 {
					continue
				}
				eventCategory, ok := event[0].(string)
				if !ok {
					return fmt.Errorf("failed to assert type of eventCategory to string")
				}
				eventName, ok := event[1].(string)
				if !ok {
					return fmt.Errorf("failed to assert type of eventName to string")
				}
				eventHandler, ok := event[2].(func(*LogMessage) error)
				if !ok {
					return fmt.Errorf("failed to assert type of eventHandler to function")
				}
				if logMsg.Category == eventCategory && strings.HasPrefix(logMsg.Content, eventName) {
					err = eventHandler(logMsg)
					if err != nil {
						return fmt.Errorf("failed to handle event: %s", err)
					}
				}
			}
		}
		logFile.Close()
		for {
			stat, err := os.Stat(logFileName)
			if err != nil {
				return fmt.Errorf("failed to retrieve log file info: %s", err)
			}
			currentModTime := stat.ModTime()
			if lastModTime.Unix() != currentModTime.Unix() {
				lastModTime = currentModTime
				break
			}
			time.Sleep(time.Second)
		}
		lastSize = currentSize
	}
}
