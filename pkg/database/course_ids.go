package database

import (
	"context"
	"errors"
	"fmt"
	"kafka-log-processor/pkg/models"
	"log"
	"reflect"

	"github.com/olivere/elastic"
	edxparser "github.com/veotani/edx-structure-json"
)

// GetUniqueStringFieldValuesInIndex returns all possible values of field fieldName in index indexName
// Field fieldName should have type keyword (string)
func (es *ElasticService) GetUniqueStringFieldValuesInIndex(indexName string, fieldName string) ([]string, error) {
	if es.client == nil {
		return nil, errors.New("You need to connect to ElasticSearch first")
	}
	searchResults, err := es.client.Search().
		Index(indexName).
		Aggregation("custom_aggregation", elastic.NewTermsAggregation().
			Field(fieldName)).
		Do(context.Background())
	if err != nil {
		return nil, err
	}
	aggregationResults, ok := searchResults.Aggregations.Terms("custom_aggregation")
	if !ok {
		return nil, errors.New("Nothing was found")
	}
	result := make([]string, 0)
	for _, bucket := range aggregationResults.Buckets {
		if fieldValue, ok := bucket.Key.(string); ok {
			result = append(result, fieldValue)
		} else {
			return nil, fmt.Errorf("Field %v should be a keyword", fieldName)
		}
	}
	return result, nil
}

// GetAllCourseCodesWithStructure returns all possible course_code values in course structures index
func (es *ElasticService) GetAllCourseCodesWithStructure() ([]string, error) {
	if es.client == nil {
		return nil, errors.New("You need to connect to ElasticSearch first")
	}
	searchResult, err := es.client.
		Search().
		Index(CourseStructureIndexName).
		Query(elastic.NewMatchAllQuery()).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]string, 0)
	var course edxparser.Course
	for _, c := range searchResult.Each(reflect.TypeOf(course)) {
		if currentCourse, ok := c.(edxparser.Course); ok {
			result = append(result, currentCourse.CourseCode)
		} else {
			return nil, errors.New("Couldn't parse course structure index")
		}
	}

	return result, nil
}

// GetAllCourseIDsWithStructureAndLogs gets all course ids met in logs, then
// scans for courses in structure index and returns their union.
// The returned type is in "course-v1:org+CourseCode+CourseRun" notation.
func (es *ElasticService) GetAllCourseIDsWithStructureAndLogs() ([]string, error) {
	courseStructureCourses, err := es.GetAllCourseCodesWithStructure()
	if err != nil {
		return nil, err
	}
	videoEventsCourses, err := es.GetUniqueStringFieldValuesInIndex(VideoEventDescriptionIndexName, "course_id")
	if err != nil {
		return nil, err
	}
	problemEventsCourses, err := es.GetUniqueStringFieldValuesInIndex(ProblemEventDescriptionIndexName, "course_id")
	if err != nil {
		return nil, err
	}

	log.Println(courseStructureCourses)
	log.Println(videoEventsCourses)

	result := make([]string, 0)

	for _, videoEventsCourse := range videoEventsCourses {
		for _, coursecourseStructureCourse := range courseStructureCourses {
			videoEventsCourseCode, err := models.GetCourseCodeFromCourseID(videoEventsCourse)
			if err != nil {
				log.Println("Skipping video event because it had invalid course_id. Please check the data!")
				continue
			}
			if coursecourseStructureCourse == videoEventsCourseCode {
				result = append(result, videoEventsCourse)
			}
		}
	}

	for _, problemEventsCourse := range problemEventsCourses {
		for _, coursecourseStructureCourse := range courseStructureCourses {
			problemEventsCourseCode, err := models.GetCourseCodeFromCourseID(problemEventsCourse)
			if err != nil {
				log.Println("Skipping problem event because it had invalid course_id. Please check the data!")
				continue
			}
			if coursecourseStructureCourse == problemEventsCourseCode {
				result = append(result, problemEventsCourse)
			}
		}
	}

	return result, nil
}