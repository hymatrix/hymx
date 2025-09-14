package chainkit

import goarSchema "github.com/permadao/goar/schema"

func (c *Chainkit) downloads(itemsIds []string) (items []*goarSchema.BundleItem, err error) {
	items, err = c.operator.Downloads(itemsIds)
	return items, err
}
