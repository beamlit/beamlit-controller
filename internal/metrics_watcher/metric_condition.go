package metrics_watcher

import (
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

type metricConditionStatus struct {
	currentMetricReachedMetrics []autoscalingv2.MetricSpec
	reached                     bool
	conditionDuration           time.Duration // The duration for which the condition must be reached to trigger the action
	sinceTime                   time.Time     // The time when the condition was first reached
}

// update updates the metric condition status
func (mcs *metricConditionStatus) update(currentMetric autoscalingv2.MetricSpec, reached bool) {

	defer func() {
		if len(mcs.currentMetricReachedMetrics) == 0 {
			mcs.reached = false
			mcs.sinceTime = time.Time{}
		}
	}()

	for i, metric := range mcs.currentMetricReachedMetrics {
		isEqual := IsEqual(metric, currentMetric)
		if isEqual && !reached {
			// Remove the metric from the list if the condition is no more reached
			mcs.currentMetricReachedMetrics = append(mcs.currentMetricReachedMetrics[:i], mcs.currentMetricReachedMetrics[i+1:]...)
			return
		}
		// If the metric is reached and the condition is already reached, do nothing
		if isEqual && reached {
			return
		}
	}

	// If the condition is reached, and it was not reached before, update the condition status
	if reached && !mcs.reached {
		mcs.currentMetricReachedMetrics = append(mcs.currentMetricReachedMetrics, currentMetric)
		mcs.reached = true
		mcs.sinceTime = time.Now()
		return
	}

	// If the condition is already reached, do nothing, just add the metric to the list
	if reached && mcs.reached {
		mcs.currentMetricReachedMetrics = append(mcs.currentMetricReachedMetrics, currentMetric)
		return
	}

}

// isReached returns true if the condition is reached, false otherwise
func (mcs *metricConditionStatus) isReached() bool {
	if !mcs.reached {
		return false
	}
	if time.Since(mcs.sinceTime) > mcs.conditionDuration {
		return true
	}
	return false
}
