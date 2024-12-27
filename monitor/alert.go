package monitor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

type DingRobot struct {
	RobotId string
}
type ParamCronTask struct {
	At struct {
		AtMobiles []struct {
			AtMobile string `json:"atMobile"`
		} `json:"atMobiles"` //At和Tele是一对多关系
		IsAtAll bool `json:"isAtAll"`
	} `json:"at"` //存储的是@的成员
	Text struct {
		Content string `json:"content"`
	} `json:"text"` //存储的是用户所发送的信息
	Msgtype string `json:"msgtype"`
}

func (t *DingRobot) SendMessage(p *ParamCronTask) error {
	b := []byte{}
	//我们需要在文本，链接，markdown三种其中的一个
	if p.Msgtype == "text" {
		msg := map[string]interface{}{}
		atMobileStringArr := make([]string, len(p.At.AtMobiles))
		for i, atMobile := range p.At.AtMobiles {
			atMobileStringArr[i] = atMobile.AtMobile
		}

		msg = map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": p.Text.Content,
			},
		}
		if p.At.IsAtAll {
			msg["at"] = map[string]interface{}{
				"isAtAll": p.At.IsAtAll,
			}
		} else {
			msg["at"] = map[string]interface{}{
				"atMobiles": atMobileStringArr, //字符串切片类型

				"isAtAll": p.At.IsAtAll,
			}
		}
		b, _ = json.Marshal(msg)
	}
	var resp *http.Response
	var err error

	resp, err = http.Post(t.getURLV2(), "application/json", bytes.NewBuffer(b))

	if err != nil {
		return err
	}
	defer resp.Body.Close()
	date, err := ioutil.ReadAll(resp.Body)
	r := struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}{}
	err = json.Unmarshal(date, &r)
	if err != nil {
		return err
	}
	if r.Errcode != 0 {
		fmt.Println(r.Errmsg)
		return errors.New(r.Errmsg)
	}

	return nil
}
func (t *DingRobot) getURLV2() string {
	url := "https://oapi.dingtalk.com/robot/send?access_token=" + t.RobotId //拼接token路径
	return url
}
