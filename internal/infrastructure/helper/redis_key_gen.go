package helper

import "fmt"

func Key(domain, entity string, identifier any) string {
	return fmt.Sprintf("%s:%s:%v", domain, entity, identifier)
}
