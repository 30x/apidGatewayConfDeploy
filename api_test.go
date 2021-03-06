// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package apiGatewayConfDeploy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/apid/apid-core/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	apiTestUrl = "http://127.0.0.1:9000"
	testBlobId = "gcs:SHA-512:39ca7ae89bb9468af34df8bc873748b4035210c91bcc01359c092c1d51364b5f3df06bc69a40621acfaa46791af9ea41bc0f3429a84738ba1a7c8d394859601a"
)

var _ = Describe("api", func() {
	var testCount int
	var dummyDbMan *dummyDbManager
	var testApiMan *apiManager

	var _ = BeforeEach(func() {
		testCount += 1
		dummyDbMan = &dummyDbManager{
			lsn: "0.1.1",
		}
		testApiMan = &apiManager{
			dbMan: dummyDbMan,
			configurationEndpoint:   configEndpoint + strconv.Itoa(testCount),
			blobEndpoint:            blobEndpointPath + strconv.Itoa(testCount) + "/{blobId}",
			configurationIdEndpoint: configEndpoint + strconv.Itoa(testCount) + "/{configId}",
			newChangeListChan:       make(chan interface{}, 5),
			addSubscriber:           make(chan chan interface{}),
		}
		testApiMan.InitAPI()
		time.Sleep(100 * time.Millisecond)
	})

	var _ = AfterEach(func() {
		testApiMan = nil
	})
	Context("GET /configurations", func() {
		It("should get empty set if no deployments", func() {
			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = configEndpoint + strconv.Itoa(testCount)

			// http get
			res, err := http.Get(uri.String())
			Expect(err).Should(Succeed())
			defer res.Body.Close()
			Expect(res.StatusCode).Should(Equal(http.StatusOK))

			// parse response
			var depRes ApiConfigurationResponse
			body, err := ioutil.ReadAll(res.Body)
			Expect(err).Should(Succeed())
			err = json.Unmarshal(body, &depRes)
			Expect(err).Should(Succeed())

			// verify response
			Expect(len(depRes.ApiConfigurationsResponse)).To(Equal(0))
			Expect(depRes.Kind).Should(Equal(kindCollection))
			Expect(depRes.Self).Should(Equal(apiTestUrl + configEndpoint + strconv.Itoa(testCount)))

		})

		It("should get correct config format", func() {
			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = configEndpoint + strconv.Itoa(testCount)

			// set test data
			details := setTestDeployments(dummyDbMan, uri.String())

			// http get
			res, err := http.Get(uri.String())
			Expect(err).Should(Succeed())
			defer res.Body.Close()
			Expect(res.StatusCode).Should(Equal(http.StatusOK))

			// parse response
			var depRes ApiConfigurationResponse
			body, err := ioutil.ReadAll(res.Body)
			Expect(err).Should(Succeed())
			err = json.Unmarshal(body, &depRes)
			Expect(err).Should(Succeed())

			// verify response
			Expect(depRes.Kind).Should(Equal(kindCollection))
			Expect(depRes.Self).Should(Equal(uri.String()))
			Expect(depRes.ApiConfigurationsResponse).Should(Equal(details))

		})

		It("should get configs by filter", func() {
			typeFilter := "ORGANIZATION"
			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = configEndpoint + strconv.Itoa(testCount)

			query := uri.Query()
			query.Add("type", typeFilter)
			uri.RawQuery = query.Encode()
			// set test data
			dep := makeTestDeployment()

			dummyDbMan.configurations = make(map[string]*Configuration)
			dummyDbMan.configurations[typeFilter] = dep
			detail := makeExpectedDetail(dep, strings.Split(uri.String(), "?")[0])

			// http get
			res, err := http.Get(uri.String())
			Expect(err).Should(Succeed())
			defer res.Body.Close()
			Expect(res.StatusCode).Should(Equal(http.StatusOK))

			// parse response
			var depRes ApiConfigurationResponse
			body, err := ioutil.ReadAll(res.Body)
			Expect(err).Should(Succeed())
			err = json.Unmarshal(body, &depRes)
			Expect(err).Should(Succeed())

			// verify response
			Expect(depRes.Kind).Should(Equal(kindCollection))
			Expect(depRes.Self).Should(Equal(uri.String()))
			Expect(depRes.ApiConfigurationsResponse).Should(Equal([]ApiConfigurationDetails{*detail}))

		})

		It("should not long poll if using filter", func() {
			typeFilter := "ORGANIZATION"
			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = configEndpoint + strconv.Itoa(testCount)

			query := uri.Query()
			query.Add("type", typeFilter)
			query.Add("block", "3")
			query.Add(apidConfigIndexPar, dummyDbMan.lsn)
			uri.RawQuery = query.Encode()
			// set test data
			dep := makeTestDeployment()

			dummyDbMan.configurations = make(map[string]*Configuration)
			dummyDbMan.configurations[typeFilter] = dep
			detail := makeExpectedDetail(dep, strings.Split(uri.String(), "?")[0])

			// http get
			res, err := http.Get(uri.String())
			Expect(err).Should(Succeed())
			defer res.Body.Close()
			Expect(res.StatusCode).Should(Equal(http.StatusOK))

			// parse response
			var depRes ApiConfigurationResponse
			body, err := ioutil.ReadAll(res.Body)
			Expect(err).Should(Succeed())
			err = json.Unmarshal(body, &depRes)
			Expect(err).Should(Succeed())

			// verify response
			Expect(depRes.Kind).Should(Equal(kindCollection))
			Expect(depRes.Self).Should(Equal(strings.Split(uri.String(), "?")[0] + "?type=" + typeFilter))
			Expect(depRes.ApiConfigurationsResponse).Should(Equal([]ApiConfigurationDetails{*detail}))

		}, 1)

		It("should get 304 for no change", func() {

			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = configEndpoint + strconv.Itoa(testCount)

			// set test data
			setTestDeployments(dummyDbMan, uri.String())
			// http get
			res, err := http.Get(uri.String())
			Expect(err).Should(Succeed())
			defer res.Body.Close()
			Expect(res.StatusCode).Should(Equal(http.StatusOK))
			lsn := res.Header.Get(apidConfigIndexHeader)
			Expect(lsn).ShouldNot(BeEmpty())

			// send second request
			query := uri.Query()
			query.Add(apidConfigIndexPar, lsn)
			uri.RawQuery = query.Encode()
			log.Debug(uri.String())
			req, err := http.NewRequest("GET", uri.String(), nil)
			req.Header.Add("Content-Type", "application/json")

			// get response
			res, err = http.DefaultClient.Do(req)
			Expect(err).ShouldNot(HaveOccurred())
			defer res.Body.Close()
			Expect(res.StatusCode).To(Equal(http.StatusNotModified))
		})

		// block is not enabled now
		It("should do long-polling if Gateway_LSN>=APID_LSN, should get 304 for timeout", func() {

			start := time.Now()

			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = configEndpoint + strconv.Itoa(testCount)
			query := uri.Query()
			query.Add("block", "1")
			query.Add(apidConfigIndexPar, "1.0.0")
			uri.RawQuery = query.Encode()

			// http get
			res, err := http.Get(uri.String())
			Expect(err).Should(Succeed())
			defer res.Body.Close()
			Expect(res.StatusCode).Should(Equal(http.StatusNotModified))

			//verify blocking time
			blockingTime := time.Since(start)
			Expect(blockingTime.Seconds() > 0.9).Should(BeTrue())

		}, 2)

		It("should do long-polling if Gateway_LSN>=APID_LSN, should get 200 if not timeout", func() {

			testLSN := fmt.Sprintf("%d.%d.%d", testCount, testCount, testCount)
			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = configEndpoint + strconv.Itoa(testCount)
			query := uri.Query()
			query.Add("block", "2")
			query.Add(apidConfigIndexPar, "1.0.0")
			uri.RawQuery = query.Encode()
			// set test data
			details := setTestDeployments(dummyDbMan, strings.Split(uri.String(), "?")[0])

			// notify change
			go func() {
				time.Sleep(time.Second)
				dummyDbMan.lsn = testLSN
				testApiMan.notifyNewChange()
			}()

			// http get
			res, err := http.Get(uri.String())
			Expect(err).Should(Succeed())
			defer res.Body.Close()
			Expect(res.StatusCode).Should(Equal(http.StatusOK))
			Expect(res.Header.Get(apidConfigIndexHeader)).Should(Equal(testLSN))
			// parse response
			var depRes ApiConfigurationResponse
			body, err := ioutil.ReadAll(res.Body)
			Expect(err).Should(Succeed())
			err = json.Unmarshal(body, &depRes)
			Expect(err).Should(Succeed())

			// verify response
			Expect(depRes.Kind).Should(Equal(kindCollection))
			Expect(depRes.Self).Should(Equal(strings.Split(uri.String(), "?")[0]))
			Expect(depRes.ApiConfigurationsResponse).Should(Equal(details))
		}, 3)

		It("should support long-polling for multiple subscribers", func() {

			testLSN := fmt.Sprintf("%d.%d.%d", testCount, testCount, testCount)
			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = configEndpoint + strconv.Itoa(testCount)
			query := uri.Query()
			query.Add("block", "3")
			query.Add(apidConfigIndexPar, dummyDbMan.lsn)
			uri.RawQuery = query.Encode()

			// set test data
			setTestDeployments(dummyDbMan, strings.Split(uri.String(), "?")[0])

			// http get
			count := mathrand.Intn(20) + 5
			finishChan := make(chan int)
			for i := 0; i < count; i++ {
				go func() {
					defer GinkgoRecover()
					res, err := http.Get(uri.String())
					Expect(err).Should(Succeed())
					defer res.Body.Close()
					finishChan <- res.StatusCode
				}()
			}

			// notify change
			go func() {
				time.Sleep(1500 * time.Millisecond)
				dummyDbMan.lsn = testLSN
				testApiMan.notifyNewChange()
			}()

			for i := 0; i < count; i++ {
				Expect(<-finishChan).Should(Equal(http.StatusOK))
			}

		}, 5)

		It("should get iso8601 time", func() {
			testTimes := []string{"", "2017-04-05 04:47:36.462 +0000 UTC", "2017-04-05 04:47:36.462-07:00", "2017-04-05T04:47:36.462Z", "2017-04-05 23:23:38.162+00:00", "2017-06-22 16:41:02.334"}
			isoTime := []string{"", "2017-04-05T04:47:36.462Z", "2017-04-05T04:47:36.462-07:00", "2017-04-05T04:47:36.462Z", "2017-04-05T23:23:38.162Z", "2017-06-22T16:41:02.334Z"}

			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = configEndpoint + strconv.Itoa(testCount)

			for i, t := range testTimes {
				log.Debug("insert deployment with timestamp: " + t)
				// set test data
				dep := makeTestDeployment()
				dep.Created = t
				dep.Updated = t
				dummyDbMan.readyDeployments = []Configuration{*dep}
				detail := makeExpectedDetail(dep, uri.String())
				detail.Created = isoTime[i]
				detail.Updated = isoTime[i]
				// http get
				res, err := http.Get(uri.String())
				Expect(err).Should(Succeed())
				defer res.Body.Close()
				Expect(res.StatusCode).Should(Equal(http.StatusOK))
				// parse response
				var depRes ApiConfigurationResponse
				body, err := ioutil.ReadAll(res.Body)
				Expect(err).Should(Succeed())
				err = json.Unmarshal(body, &depRes)
				Expect(err).Should(Succeed())
				// verify response
				Expect(depRes.ApiConfigurationsResponse).Should(Equal([]ApiConfigurationDetails{*detail}))

			}
		})

	})

	Context("GET /blobs", func() {
		It("should get file bytes from endpoint", func() {
			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = blobEndpointPath + strconv.Itoa(testCount) + "/test"

			// set test data
			testFile, err := ioutil.TempFile(bundlePath, "test")
			randString := util.GenerateUUID()
			testFile.Write([]byte(randString))
			err = testFile.Close()
			Expect(err).Should(Succeed())
			dummyDbMan.localFSLocation = testFile.Name()

			log.Debug("randString: " + randString)

			// http get
			res, err := http.Get(uri.String())
			Expect(err).Should(Succeed())
			defer res.Body.Close()
			Expect(res.StatusCode).Should(Equal(http.StatusOK))

			// parse response
			body, err := ioutil.ReadAll(res.Body)
			Expect(err).Should(Succeed())
			Expect(string(body)).Should(Equal(randString))
		})

		It("should get error response", func() {
			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = blobEndpointPath + strconv.Itoa(testCount) + "/" + util.GenerateUUID()

			// set test data
			testData := [][]interface{}{
				{"", sql.ErrNoRows},
				{"", fmt.Errorf("test error")},
			}
			expectedCode := []int{
				http.StatusNotFound,
				http.StatusInternalServerError,
			}

			for i, data := range testData {
				dummyDbMan.localFSLocation = data[0].(string)
				dummyDbMan.err = data[1].(error)
				// http get
				res, err := http.Get(uri.String())
				Expect(err).Should(Succeed())
				res.Body.Close()
				Expect(res.StatusCode).Should(Equal(expectedCode[i]))
			}
		})
	})

	Context("GET /configurations/{configId}", func() {
		It("should get configuration according to {configId}", func() {
			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())
			uri.Path = configEndpoint + strconv.Itoa(testCount) + "/3ecd351c-1173-40bf-b830-c194e5ef9038"

			//setup test data
			dummyDbMan.err = nil
			dummyDbMan.configurations = make(map[string]*Configuration)
			expectedConfig := &Configuration{
				ID:             "3ecd351c-1173-40bf-b830-c194e5ef9038",
				OrgID:          "73fcac6c-5d9f-44c1-8db0-333efda3e6e8",
				EnvID:          "ada76573-68e3-4f1a-a0f9-cbc201a97e80",
				BlobID:         "gcs:SHA-512:8fcc902465ccb32ceff25fa9f6fb28e3b314dbc2874c0f8add02f4e29c9e2798d344c51807aa1af56035cf09d39c800cf605d627ba65723f26d8b9c83c82d2f2",
				BlobResourceID: "gcs:SHA-512:0c648779da035bfe0ac21f6268049aa0ae74d9d6411dadefaec33991e55c2d66c807e06f7ef84e0947f7c7d63b8c9e97cf0684cbef9e0a86b947d73c74ae7455",
				Type:           "ENVIRONMENT",
				Name:           "test",
				Revision:       "",
				Path:           "/organizations/Org1//environments/test/",
				Created:        "2017-06-27 03:14:46.018+00:00",
				CreatedBy:      "defaultUser",
				Updated:        "2017-06-27 03:14:46.018+00:00",
				UpdatedBy:      "defaultUser",
			}
			dummyDbMan.configurations[expectedConfig.ID] = expectedConfig
			// http get
			res, err := http.Get(uri.String())
			Expect(err).Should(Succeed())
			defer res.Body.Close()
			Expect(res.StatusCode).Should(Equal(http.StatusOK))

			// parse response
			var depRes ApiConfigurationDetails
			body, err := ioutil.ReadAll(res.Body)
			Expect(err).Should(Succeed())
			err = json.Unmarshal(body, &depRes)
			Expect(err).Should(Succeed())

			// verify response
			Expect(depRes.Self).Should(ContainSubstring(expectedConfig.ID))
			Expect(depRes.Org).Should(Equal(expectedConfig.OrgID))
			Expect(depRes.Name).Should(Equal(expectedConfig.Name))
			Expect(depRes.Type).Should(Equal(expectedConfig.Type))
			Expect(depRes.Revision).Should(Equal(expectedConfig.Revision))
			Expect(depRes.BeanBlobUrl).Should(ContainSubstring(expectedConfig.BlobID))
			Expect(depRes.ResourceBlobUrl).Should(ContainSubstring(expectedConfig.BlobResourceID))
			Expect(depRes.Path).Should(Equal(expectedConfig.Path))
			Expect(depRes.Created).Should(Equal(convertTime(expectedConfig.Created)))
			Expect(depRes.Updated).Should(Equal(convertTime(expectedConfig.Updated)))
		})

		It("should get error responses", func() {
			// setup http client
			uri, err := url.Parse(apiTestUrl)
			Expect(err).Should(Succeed())

			//setup test data
			testData := [][]interface{}{
				{util.GenerateUUID(), sql.ErrNoRows},
				{util.GenerateUUID(), fmt.Errorf("test error")},
			}
			expectedCode := []int{
				http.StatusNotFound,
				http.StatusInternalServerError,
			}

			for i, data := range testData {
				if data[1] != nil {
					dummyDbMan.err = data[1].(error)
				}
				dummyDbMan.configurations = make(map[string]*Configuration)
				dummyDbMan.configurations[data[0].(string)] = &Configuration{}
				// http get
				uri.Path = configEndpoint + strconv.Itoa(testCount) + "/" + data[0].(string)
				res, err := http.Get(uri.String())
				Expect(err).Should(Succeed())
				Expect(res.StatusCode).Should(Equal(expectedCode[i]))
				res.Body.Close()
			}
		})
	})

})

func setTestDeployments(dummyDbMan *dummyDbManager, self string) []ApiConfigurationDetails {

	mathrand.Seed(time.Now().UnixNano())
	count := mathrand.Intn(5) + 1
	deployments := make([]Configuration, count)
	details := make([]ApiConfigurationDetails, count)

	for i := 0; i < count; i++ {
		dep := makeTestDeployment()
		detail := makeExpectedDetail(dep, self)

		deployments[i] = *dep
		details[i] = *detail
	}

	dummyDbMan.readyDeployments = deployments

	return details
}

func makeTestDeployment() *Configuration {
	dep := &Configuration{
		ID:             util.GenerateUUID(),
		OrgID:          util.GenerateUUID(),
		EnvID:          util.GenerateUUID(),
		BlobID:         util.GenerateUUID(), //testBlobId,
		BlobResourceID: util.GenerateUUID(), //"",
		Type:           "virtual-host",
		Name:           "vh-secure",
		Revision:       "1",
		Path:           "/organizations/Org1/",
		Created:        time.Now().Format(time.RFC3339),
		CreatedBy:      "haoming@google.com",
		Updated:        time.Now().Format(time.RFC3339),
		UpdatedBy:      "haoming@google.com",
	}
	return dep
}

func makeExpectedDetail(dep *Configuration, self string) *ApiConfigurationDetails {
	detail := &ApiConfigurationDetails{
		Self:            self + "/" + dep.ID,
		Name:            dep.Name,
		Type:            dep.Type,
		Revision:        dep.Revision,
		BeanBlobUrl:     getBlobUrl(dep.BlobID),
		Org:             dep.OrgID,
		Env:             dep.EnvID,
		ResourceBlobUrl: getBlobUrl(dep.BlobResourceID),
		Path:            dep.Path,
		Created:         dep.Created,
		Updated:         dep.Updated,
	}
	return detail
}
