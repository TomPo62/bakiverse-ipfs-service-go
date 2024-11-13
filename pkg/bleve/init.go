package bleve
import (
	"github.com/blevesearch/bleve/v2"
)

func InitBleveIndex() (bleve.Index, error) {
	index, err := bleve.Open("files_index.bleve")
	if err == bleve.ErrorIndexPathDoesNotExist {
		mapping := bleve.NewIndexMapping()
		index, err = bleve.New("files_index.bleve", mapping)
	}
	return index, err
}
