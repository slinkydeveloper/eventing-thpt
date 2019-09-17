/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	vegeta "github.com/tsenart/vegeta/lib"
	"net/http"
	"strconv"
)

type CloudEventsTargeter struct {
	sinkUrl          string
	msgSize          int
	eventType        string
	eventSource      string
	encodingSelector cehttp.EncodingSelector
}

func NewCloudEventsTargeter(sinkUrl string, msgSize int, eventType string, eventSource string, encoding string) CloudEventsTargeter {
	var encodingSelector cehttp.EncodingSelector
	if encoding == "binary" {
		encodingSelector = cehttp.DefaultBinaryEncodingSelectionStrategy
	} else {
		encodingSelector = cehttp.DefaultStructuredEncodingSelectionStrategy
	}
	return CloudEventsTargeter{
		sinkUrl:          sinkUrl,
		msgSize:          msgSize,
		eventType:        eventType,
		eventSource:      eventSource,
		encodingSelector: encodingSelector,
	}
}

func (cet CloudEventsTargeter) VegetaTargeter() vegeta.Targeter {
	seq := uint64(0)

	b := GenerateRandByteArray(cet.msgSize)

	return func(t *vegeta.Target) error {
		t.Method = http.MethodPost
		t.URL = cet.sinkUrl

		t.Header = make(http.Header)

		t.Header.Set("Ce-Id", strconv.FormatUint(seq, 10))
		t.Header.Set("Ce-Type", cet.eventType)
		t.Header.Set("Ce-Source", cet.eventSource)
		t.Header.Set("Ce-Specversion", "0.2")

		t.Header.Set("Content-Type", "text/plain")

		t.Body = b

		seq++

		return nil
	}
}
