package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

type ECRAPI interface {
	DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error)
	ListImages(ctx context.Context, params *ecr.ListImagesInput, optFns ...func(*ecr.Options)) (*ecr.ListImagesOutput, error)
}

type imageDetail struct {
	name, date string
}

type ecrSearch struct {
	client  ECRAPI
	results []imageDetail
}

func NewEcrSearch(region *string) *ecrSearch {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}

	return &ecrSearch{
		client: ecr.NewFromConfig(cfg, func(o *ecr.Options) {
			o.Region = *region
		}),
	}
}

func (search *ecrSearch) sortedResults() *[]imageDetail {
	sort.Slice(search.results, func(i, j int) bool {
		return search.results[i].date > search.results[j].date
	})

	return &search.results
}

// populates search.results with details (like pushed time) from the images in i
func (search *ecrSearch) buildResults(i *[]types.ImageIdentifier, image *string) {
	input := ecr.DescribeImagesInput{
		ImageIds:       *i,
		RepositoryName: image,
	}

	// TODO: add paging @ 100 per request to match DescribeImages
	// maximum ImageIds
	result, err := search.client.DescribeImages(context.TODO(), &input)
	if err != nil {
		fmt.Println(err)
	}

	for _, image := range result.ImageDetails {
		for i := range image.ImageTags {
			id := imageDetail{
				name: image.ImageTags[i],
				date: image.ImagePushedAt.String(),
			}

			search.results = append(search.results, id)
		}
	}
}

func (search *ecrSearch) print(image *string) {
	var (
		minwidth, tabwidth, padding int  = 0, 0, 1
		padchar                     byte = ' '
		flags                       uint = 0
	)
	w := tabwriter.NewWriter(os.Stdout, minwidth, tabwidth, padding, padchar, flags)
	for _, tag := range *search.sortedResults() {
		fmt.Fprintf(w, "%v:%v\t\t%v\n", *image, tag.name, tag.date)
	}
	w.Flush()
}

func (search *ecrSearch) getAllTags(image *string) (result *ecr.ListImagesOutput) {
	var (
		filter     = &types.ListImagesFilter{TagStatus: types.TagStatusTagged}
		maxResults = int32(1000)
	)

	input := &ecr.ListImagesInput{
		RepositoryName: aws.String(*image),
		Filter:         filter,
		MaxResults:     &maxResults,
	}

	result, err := search.client.ListImages(context.TODO(), input)
	if err != nil {
		panic(err)
	}
	return
}

// returns a slice of types.ImageIdentifiers for all tags matching the regex
func (search *ecrSearch) findTags(regex, image *string) (i []types.ImageIdentifier) {
	for _, tag := range search.getAllTags(image).ImageIds {
		matched, _ := regexp.MatchString(*regex, *tag.ImageTag)
		if matched {
			i = append(i, tag)
		}
	}
	return
}

func main() {
	var image, region, regex string

	flag.StringVar(&image, "image", "", "The image name to search for")
	flag.StringVar(&region, "region", "us-east-1", "The AWS region to use")
	flag.StringVar(&regex, "regex", "^latest", "Regex used to filter tags")
	flag.Parse()

	search := NewEcrSearch(&region)
	tags := search.findTags(&regex, &image)
	search.buildResults(&tags, &image)
	search.print(&image)
}
