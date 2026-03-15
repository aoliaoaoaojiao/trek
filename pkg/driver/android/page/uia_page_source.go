package page

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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

	url := fmt.Sprintf("%s/session/%s/source", u.RemoteUrl, u.SessionId)
	resp, err := u.Request(http.MethodGet, url, nil, 60*time.Second)
	if err != nil {
		return "", err
	}

	var pageSource string
	if err := json.Unmarshal(resp.Value, &pageSource); err != nil {
		// 如果不是字符串，直接返回原始值
		return string(resp.Value), nil
	}

	return pageSource, nil
}

func (u *UIAPageSource) Close() error {
	u.UiaClient = nil
	return nil
}
