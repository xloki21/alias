package link

import (
	"fmt"
	"github.com/xloki21/alias/internal/domain"
	"math/rand"
	"net/url"
)

func TestURLSet(quantity int) []*domain.Alias {
	aliases := make([]*domain.Alias, quantity)
	ttlValue := rand.Intn(10)
	isPermanent := false
	if ttlValue == 0 {
		isPermanent = true
	}
	rawUrlString := fmt.Sprintf("http://very-long-url-%d.test/path/to/somewhere", rand.Int())
	validURL, _ := url.Parse(rawUrlString)

	for i := 0; i < quantity; i++ {
		aliases[i] = &domain.Alias{
			Origin:      validURL,
			TTL:         ttlValue,
			IsActive:    true,
			IsPermanent: isPermanent,
		}
	}
	return aliases
}
