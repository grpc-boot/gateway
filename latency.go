package gateway

import "time"

type LatencyList []time.Duration

func (ll LatencyList) Len() int {
	return len(ll)
}

func (ll LatencyList) Swap(i, j int) {
	ll[i], ll[j] = ll[j], ll[i]
}

func (ll LatencyList) Less(i, j int) bool {
	return ll[i] < ll[j]
}

func (ll LatencyList) Min() time.Duration {
	return ll[0]
}

func (ll LatencyList) Max() time.Duration {
	return ll[len(ll)-1]
}

func (ll LatencyList) Avg() time.Duration {
	var sum time.Duration

	for _, v := range ll {
		sum += v
	}

	return sum / time.Duration(len(ll))
}

func (ll LatencyList) L90() time.Duration {
	index := int(float32(len(ll)) * 0.9)
	if index == len(ll) {
		return ll.Max()
	}
	return ll[index]
}

func (ll LatencyList) L95() time.Duration {
	index := int(float32(len(ll)) * 0.95)
	if index == len(ll) {
		return ll.Max()
	}
	return ll[index]
}
