package page

import (
	"encoding/json"
	"net/http"
	"time"
	"trek/logger"
	"trek/pkg/driver/android/uia"
	"trek/pkg/driver/common"
)

const (
	defaultFindElementInterval = 3000
	defaultFindElementRetry    = 5
)

// NewUIAPageSource 创建 UIA 页面获取实例
func NewUIAPageSource(client *uia.UiaClient) *UIAPageSource {
	return &UIAPageSource{
		UiaClient:           client,
		findElementInterval: defaultFindElementInterval,
		findElementRetry:    defaultFindElementRetry,
	}
}

var _ common.IPageSource = (*UIAPageSource)(nil)

type UIAPageSource struct {
	findElementInterval int
	findElementRetry    int
	*uia.UiaClient
}

// SetFindElementConfig 设置查找元素的配置
func (u *UIAPageSource) SetFindElementConfig(retry, interval int) {
	if retry > 0 {
		u.findElementRetry = retry
	}
	if interval > 0 {
		u.findElementInterval = interval
	}
}

// DumpPageSource 获取页面源码
func (u *UIAPageSource) DumpPageSource() (string, error) {
	if u.UiaClient == nil {
		return "", common.NoUIAClientErr
	}
	if err := u.CheckSessionId(); err != nil {
		return "", err
	}
	logger.Debugf("Starting UIA page source dump, sessionId=%s", u.SessionId)

	resp, err := u.Request(http.MethodGet, u.SessionURL("/source"), nil, 60*time.Second)
	if err != nil {
		return "", err
	}

	var pageSource string
	if err := json.Unmarshal(resp.Value, &pageSource); err != nil {
		return string(resp.Value), nil
	}

	logger.Debugf("UIA page source dump completed, sessionId=%s size=%d", u.SessionId, len(pageSource))
	return pageSource, nil
}

func (u *UIAPageSource) Close() error {
	u.UiaClient = nil
	return nil
}
