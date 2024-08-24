package controller

type requestSingleURL struct {
	URL string `json:"url"`
}

type requestURLList struct {
	URLs []string `json:"urls"`
}
