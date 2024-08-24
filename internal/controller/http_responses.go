package controller

type responseSingleURL struct {
	URL string `json:"url"`
}

type responseURLList struct {
	URLs []string `json:"urls"`
}
