package writer

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/yuriadams/prometheus-kafka-adapter/config"
)

// Handle receives the payload from Prometheus, format and send to Kafka
func Handle(w http.ResponseWriter, r *http.Request) {
	compressed, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reqBuf, err := snappy.Decode(nil, compressed)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req prompb.WriteRequest
	if err := proto.Unmarshal(reqBuf, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	samples := protoToSamples(&req)
	config.ReceivedSamples.Add(float64(len(samples)))

	writer := config.BuildClient()
	go func(rw config.Writer) {
		sendSamples(rw, samples)
	}(writer)

}

func protoToSamples(req *prompb.WriteRequest) model.Samples {
	var samples model.Samples
	for _, ts := range req.Timeseries {
		metric := make(model.Metric, len(ts.Labels))
		for _, l := range ts.Labels {
			metric[model.LabelName(l.Name)] = model.LabelValue(l.Value)
		}

		for _, s := range ts.Samples {
			samples = append(samples, &model.Sample{
				Metric:    metric,
				Value:     model.SampleValue(s.Value),
				Timestamp: model.Time(s.Timestamp),
			})

		}
	}
	return samples
}

func sendSamples(w config.Writer, samples model.Samples) {
	// begin := time.Now()

	err := w.Write(samples)

	fmt.Printf("%+v\n", err)
	// duration := time.Since(begin).Seconds()
	// if err != nil {
	// 	log.With("num_samples", len(samples)).With("storage", w.Name()).With("err", err).Warnf("Error sending samples to remote storage")
	// 	config.FailedSamples.WithLabelValues(w.Name()).Add(float64(len(samples)))
	// }
	// config.SentSamples.WithLabelValues(w.Name()).Add(float64(len(samples)))
	// config.SentBatchDuration.WithLabelValues(w.Name()).Observe(duration)
}
