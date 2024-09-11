package controller

type requestDeleteAlias struct {
	Key string `json:"key"`
}

type requestURLList struct {
	URLs []string `json:"urls"`
}
