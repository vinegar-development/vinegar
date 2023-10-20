package roblox

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/vinegarhq/vinegar/util"
)

const (
	DefaultChannel    = "live"
	ClientSettingsURL = "https://clientsettingscdn.roblox.com/v2/client-version"
)

var ErrNoVersion = errors.New("no version found")

type ClientVersion struct {
	Version                 string `json:"version"`
	ClientVersionUpload     string `json:"clientVersionUpload"`
	BootstrapperVersion     string `json:"bootstrapperVersion"`
	NextClientVersionUpload string `json:"nextClientVersionUpload"`
	NextClientVersion       string `json:"nextClientVersion"`
}

type Version struct {
	Type    BinaryType
	Channel string
	GUID    string
}

func (v Version) String() string {
	return v.Channel + " " + v.GUID
}

func ChannelPath(channel string) string {
	// Ensure that the channel is lowercased, since internally in
	// ClientSettings it will be lowercased, but not on the deploy mirror.
	channel = strings.ToLower(channel)

	if channel == DefaultChannel {
		return "/"
	}

	return "/channel/" + channel + "/"
}

func NewVersion(bt BinaryType, channel string, GUID string) (Version, error) {
	if channel == "" {
		channel = DefaultChannel
	}

	if GUID == "" {
		return Version{}, ErrNoVersion
	}

	log.Printf("Found %s version %s", bt.BinaryName(), GUID)

	return Version{
		Type:    bt,
		Channel: channel,
		GUID:    GUID,
	}, nil
}

func LatestVersion(bt BinaryType, channel string) (Version, error) {
	name := bt.BinaryName()
	var cv ClientVersion

	if channel == "" {
		channel = DefaultChannel
	}

	url := ClientSettingsURL + "/" + name + ChannelPath(channel)

	log.Printf("Fetching latest version of %s for channel %s (%s)", name, channel, url)

	resp, err := util.Body(url)
	if err != nil {
		if errors.Is(err, util.ErrBadStatus) {
			return Version{}, fmt.Errorf("invalid channel given for %s", name)
		} else {
			return Version{}, fmt.Errorf("fetch version for %s: %w", name, err)
		}
	}

	err = json.Unmarshal([]byte(resp), &cv)
	if err != nil {
		return Version{}, fmt.Errorf("version clientsettings unmarshal: %w", err)
	}

	if cv.ClientVersionUpload == "" {
		return Version{}, ErrNoVersion
	}

	log.Printf("Fetched %s canonical version %s", bt.BinaryName(), cv.Version)

	return NewVersion(bt, channel, cv.ClientVersionUpload)
}
