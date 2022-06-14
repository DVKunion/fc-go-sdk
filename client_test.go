package fc

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

var endPoint string = os.Getenv("ENDPOINT")
var accessKeyId string = os.Getenv("ACCESS_KEY_ID")
var accessKeySecret string = os.Getenv("ACCESS_KEY_SECRET")
var codeBucketName string = os.Getenv("CODE_BUCKET")
var region string = os.Getenv("REGION")
var accountID string = os.Getenv("ACCOUNT_ID")
var invocationRole string = os.Getenv("INVOCATION_ROLE")
var logProject string = os.Getenv("LOG_PROJECT")
var logStore string = os.Getenv("LOG_STORE")

type FcClientTestSuite struct {
	suite.Suite
}

func TestFcClient(t *testing.T) {
	suite.Run(t, new(FcClientTestSuite))
}

func (s *FcClientTestSuite) TestService() {
	assert := s.Require()
	prefix := RandStringBytes(8)
	serviceName := fmt.Sprintf("go-service-%s-%s", prefix, RandStringBytes(8))
	serviceNamePrefix := fmt.Sprintf("go-service-%s", prefix)

	client, err := NewClient(endPoint, "2016-08-15", accessKeyId, accessKeySecret)
	assert.Nil(err)

	listServices, err := client.ListServices(NewListServicesInput().WithLimit(100).WithPrefix(serviceNamePrefix))
	assert.Nil(err)
	for _, serviceMetadata := range listServices.Services {
		s.clearService(client, *serviceMetadata.ServiceName)
	}

	// clear
	defer func() {
		listServices, err := client.ListServices(NewListServicesInput().WithLimit(100).WithPrefix(serviceNamePrefix))
		assert.Nil(err)
		for _, serviceMetadata := range listServices.Services {
			s.clearService(client, *serviceMetadata.ServiceName)
		}
	}()

	// CreateService
	createServiceOutput, err := client.CreateService(NewCreateServiceInput().
		WithServiceName(serviceName).
		WithDescription("this is a service test for go sdk"))

	assert.Nil(err)
	assert.Equal(*createServiceOutput.ServiceName, serviceName)
	assert.Equal(*createServiceOutput.Description, "this is a service test for go sdk")
	assert.NotNil(*createServiceOutput.CreatedTime)
	assert.NotNil(*createServiceOutput.LastModifiedTime)
	assert.NotNil(*createServiceOutput.LogConfig)
	assert.NotNil(*createServiceOutput.Role)
	assert.NotNil(*createServiceOutput.ServiceID)

	// GetService
	getServiceOutput, err := client.GetService(NewGetServiceInput(serviceName))
	assert.Nil(err)

	assert.Equal(*getServiceOutput.ServiceName, serviceName)
	assert.Equal(*getServiceOutput.Description, "this is a service test for go sdk")

	// UpdateService
	updateServiceInput := NewUpdateServiceInput(serviceName).WithDescription("new description")
	updateServiceOutput, err := client.UpdateService(updateServiceInput)
	assert.Nil(err)
	assert.Equal(*updateServiceOutput.Description, "new description")

	// UpdateService with IfMatch
	updateServiceInput2 := NewUpdateServiceInput(serviceName).WithDescription("new description2").
		WithIfMatch(updateServiceOutput.Header.Get("ETag"))
	updateServiceOutput2, err := client.UpdateService(updateServiceInput2)
	assert.Nil(err)
	assert.Equal(*updateServiceOutput2.Description, "new description2")

	// UpdateService with wrong IfMatch
	updateServiceInput3 := NewUpdateServiceInput(serviceName).WithDescription("new description2").
		WithIfMatch("1234")
	_, errNoMatch := client.UpdateService(updateServiceInput3)
	assert.NotNil(errNoMatch)

	// ListServices
	listServicesOutput, err := client.ListServices(NewListServicesInput().WithLimit(100).WithPrefix(serviceNamePrefix))
	assert.Nil(err)
	assert.Equal(len(listServicesOutput.Services), 1)
	assert.Equal(*listServicesOutput.Services[0].ServiceName, serviceName)

	serviceNameArr := []string{}
	for a := 0; a < 10; a++ {
		listServiceName := fmt.Sprintf("go-service-%s-%s", prefix, RandStringBytes(8))
		_, errListService := client.CreateService(NewCreateServiceInput().
			WithServiceName(listServiceName).
			WithDescription("this is a service test for go sdk"))
		assert.Nil(errListService)

		tags := map[string]string{
			"k3": "v3",
		}
		if a%2 == 0 {
			tags["k1"] = "v1"
		} else {
			tags["k2"] = "v2"
		}
		resArn := fmt.Sprintf("services/%s", listServiceName)
		resp, err := client.TagResource(NewTagResourceInput(resArn, tags))
		assert.Nil(err)
		assert.NotNil(resp)
		assert.NotEmpty(resp.GetRequestID())

		listServicesOutput, err := client.ListServices(NewListServicesInput().WithLimit(100).WithPrefix(serviceNamePrefix))
		assert.Nil(err)
		assert.Equal(len(listServicesOutput.Services), a+2)

		serviceNameArr = append(serviceNameArr, listServiceName)
	}
	for i, svr := range serviceNameArr {
		resArn := fmt.Sprintf("services/%s", svr)
		resp, err := client.GetResourceTags(NewGetResourceTagsInput(resArn))
		assert.Nil(err)
		assert.NotNil(resp)
		assert.NotEmpty(resp.GetRequestID())
		assert.Equal(fmt.Sprintf("acs:fc:%s:%s:services/%s", region, accountID, svr), *resp.ResourceArn)
		if i%2 == 0 {
			assert.True(reflect.DeepEqual(map[string]string{
				"k1": "v1",
				"k3": "v3",
			}, resp.Tags))

		} else {
			assert.True(reflect.DeepEqual(map[string]string{
				"k2": "v2",
				"k3": "v3",
			}, resp.Tags))
		}
	}

	listServicesOutput, err = client.ListServices(NewListServicesInput().WithLimit(100).WithPrefix(serviceNamePrefix).WithTags(map[string]string{
		"k3": "v3",
	}))
	assert.Nil(err)
	assert.Equal(len(listServicesOutput.Services), 10)

	listServicesOutput, err = client.ListServices(NewListServicesInput().WithLimit(100).WithPrefix(serviceNamePrefix).WithTags(map[string]string{
		"k1": "v1",
	}))
	assert.Nil(err)
	assert.Equal(len(listServicesOutput.Services), 5)

	listServicesOutput, err = client.ListServices(NewListServicesInput().WithLimit(100).WithPrefix(serviceNamePrefix).WithTags(map[string]string{
		"k1": "v1",
		"k2": "v2",
	}))
	assert.Nil(err)
	assert.Equal(len(listServicesOutput.Services), 0)

	for _, svr := range serviceNameArr {
		resArn := fmt.Sprintf("services/%s", svr)
		client.UnTagResource(NewUnTagResourceInput(resArn).WithTagKeys([]string{"k3"}).WithAll(false))
	}

	listServicesOutput, err = client.ListServices(NewListServicesInput().WithLimit(100).WithPrefix(serviceNamePrefix).WithTags(map[string]string{
		"k3": "v3",
	}))
	assert.Nil(err)
	assert.Equal(len(listServicesOutput.Services), 0)

	for _, svr := range serviceNameArr {
		resArn := fmt.Sprintf("services/%s", svr)
		client.UnTagResource(NewUnTagResourceInput(resArn).WithTagKeys([]string{}).WithAll(true))
	}
	listServicesOutput, err = client.ListServices(NewListServicesInput().WithLimit(100).WithPrefix(serviceNamePrefix).WithTags(map[string]string{
		"k1": "v1",
	}))
	assert.Nil(err)
	assert.Equal(len(listServicesOutput.Services), 0)

	listServicesOutput, err = client.ListServices(NewListServicesInput().WithLimit(100).WithPrefix(serviceNamePrefix).WithTags(map[string]string{
		"k2": "v2",
	}))
	assert.Nil(err)
	assert.Equal(len(listServicesOutput.Services), 0)

	// DeleteService
	_, errDelService := client.DeleteService(NewDeleteServiceInput(serviceName))
	assert.Nil(errDelService)
}

func (s *FcClientTestSuite) TestFunction() {
	assert := s.Require()
	serviceName := fmt.Sprintf("go-service-%s", RandStringBytes(10))
	client, err := NewClient(endPoint, "2016-08-15", accessKeyId, accessKeySecret)

	assert.Nil(err)

	defer s.clearService(client, serviceName)

	// CreateService
	_, err2 := client.CreateService(NewCreateServiceInput().
		WithServiceName(serviceName).
		WithDescription("this is a function test for go sdk"))
	assert.Nil(err2)

	// CreateFunction
	functionName := fmt.Sprintf("go-function-%s", RandStringBytes(8))
	createFunctionInput1 := NewCreateFunctionInput(serviceName).WithFunctionName(functionName).
		WithDescription("go sdk test function").
		WithHandler("hello_world.handler").WithRuntime("nodejs6").
		WithCode(NewCode().
			WithOSSBucketName(codeBucketName).
			WithOSSObjectName("hello_world_nodejs.zip")).
		WithTimeout(5)
	createFunctionOutput, err := client.CreateFunction(createFunctionInput1)
	assert.Nil(err)

	assert.Equal(*createFunctionOutput.FunctionName, functionName)
	assert.Equal(*createFunctionOutput.Description, "go sdk test function")
	assert.Equal(*createFunctionOutput.Runtime, "nodejs6")
	assert.Equal(*createFunctionOutput.Handler, "hello_world.handler")
	assert.NotNil(*createFunctionOutput.CreatedTime)
	assert.NotNil(*createFunctionOutput.LastModifiedTime)
	assert.NotNil(*createFunctionOutput.CodeChecksum)
	assert.NotNil(*createFunctionOutput.CodeSize)
	assert.NotNil(*createFunctionOutput.FunctionID)
	assert.NotNil(*createFunctionOutput.MemorySize)
	assert.NotNil(*createFunctionOutput.Timeout)

	// GetFunction
	getFunctionOutput, err := client.GetFunction(NewGetFunctionInput(serviceName, functionName))
	assert.Nil(err)
	assert.Equal(*getFunctionOutput.FunctionName, functionName)
	assert.Equal(*getFunctionOutput.Description, "go sdk test function")
	assert.Equal(*getFunctionOutput.Runtime, "nodejs6")
	assert.Equal(*getFunctionOutput.Handler, "hello_world.handler")
	assert.Equal(*getFunctionOutput.CreatedTime, *createFunctionOutput.CreatedTime)
	assert.Equal(*getFunctionOutput.LastModifiedTime, *createFunctionOutput.LastModifiedTime)
	assert.Equal(*getFunctionOutput.CodeChecksum, *createFunctionOutput.CodeChecksum)
	assert.Equal(*createFunctionOutput.CodeSize, *createFunctionOutput.CodeSize)
	assert.Equal(*createFunctionOutput.FunctionID, *createFunctionOutput.FunctionID)
	assert.Equal(*createFunctionOutput.MemorySize, *createFunctionOutput.MemorySize)
	assert.Equal(*createFunctionOutput.Timeout, *createFunctionOutput.Timeout)

	functionName2 := fmt.Sprintf("go-function-%s", RandStringBytes(8))
	_, errReCreate := client.CreateFunction(createFunctionInput1.WithFunctionName(functionName2))
	assert.Nil(errReCreate)

	// ListFunctions
	listFunctionsOutput, err := client.ListFunctions(NewListFunctionsInput(serviceName).WithPrefix("go-function-"))
	assert.Nil(err)
	assert.Equal(len(listFunctionsOutput.Functions), 2)
	assert.True(*listFunctionsOutput.Functions[0].FunctionName == functionName || *listFunctionsOutput.Functions[1].FunctionName == functionName)
	assert.True(*listFunctionsOutput.Functions[0].FunctionName == functionName2 || *listFunctionsOutput.Functions[1].FunctionName == functionName2)

	// UpdateFunction
	updateFunctionOutput, err := client.UpdateFunction(NewUpdateFunctionInput(serviceName, functionName).
		WithDescription("newdesc"))
	assert.Equal(*updateFunctionOutput.Description, "newdesc")

	// InvokeFunction
	invokeInput := NewInvokeFunctionInput(serviceName, functionName).WithLogType("Tail")
	invokeOutput, err := client.InvokeFunction(invokeInput)
	assert.Nil(err)
	logResult, err := invokeOutput.GetLogResult()
	assert.NotNil(logResult)
	assert.NotNil(invokeOutput.GetRequestID())
	assert.Equal(string(invokeOutput.Payload), "hello world")

	invokeInput = NewInvokeFunctionInput(serviceName, functionName).WithLogType("None")
	invokeOutput, err = client.InvokeFunction(invokeInput)
	assert.NotNil(invokeOutput.GetRequestID())
	assert.Equal(string(invokeOutput.Payload), "hello world")

	// TestFunction use local zipfile
	functionName = fmt.Sprintf("go-function-%s", RandStringBytes(8))
	createFunctionInput := NewCreateFunctionInput(serviceName).WithFunctionName(functionName).
		WithDescription("go sdk test function").
		WithHandler("main.my_handler").WithRuntime("python2.7").
		WithCode(NewCode().WithFiles("./testCode/hello_world.zip")).
		WithTimeout(5)
	_, errCreateLocalFile := client.CreateFunction(createFunctionInput)
	assert.Nil(errCreateLocalFile)
	invokeOutput, err = client.InvokeFunction(invokeInput)
	assert.Nil(err)
	assert.NotNil(invokeOutput.GetRequestID())
	assert.Equal(string(invokeOutput.Payload), "hello world")

	// InvokeFunction through HTTP trigger
	functionName = fmt.Sprintf("go-function-%s", RandStringBytes(8))
	createFunctionInput = NewCreateFunctionInput(serviceName).WithFunctionName(functionName).
		WithDescription("go sdk test function").
		WithHandler("main.handler").WithRuntime("python3").
		WithCode(NewCode().WithFiles("./testCode/main.py")).
		WithTimeout(5)
	_, errCreateLocalFile = client.CreateFunction(createFunctionInput)
	assert.Nil(errCreateLocalFile)

	sourceArn := "dummy_arn"
	invocationRole := ""
	triggerName := fmt.Sprintf("go-function-trigger-%s", RandStringBytes(8))
	description := "create http trigger"
	createTriggerInput := NewCreateTriggerInput(serviceName, functionName).WithTriggerName(triggerName).
		WithDescription(description).WithInvocationRole(invocationRole).WithTriggerType("http").
		WithSourceARN(sourceArn).WithTriggerConfig(
		NewHTTPTriggerConfig().WithAuthType("function").WithMethods("GET", "POST"))

	_, err = client.CreateTrigger(createTriggerInput)
	assert.Nil(err)
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("/2016-08-15/proxy/%s/%s/", serviceName, functionName), nil)
	httpReq.Header.Set("Content-Type", "application/json")
	assert.Nil(err)
	invokeResp, err := client.DoHttpRequest(httpReq)
	assert.Nil(err)
	assert.NotNil(invokeResp)
	assert.NotNil(invokeResp.Header.Get("X-Fc-Request-Id"))
	bodyBytes, err := ioutil.ReadAll(invokeResp.Body)
	assert.Nil(err)
	assert.Equal(string(bodyBytes), "Hello world!\n")
}

func (s *FcClientTestSuite) TestTrigger() {
	assert := s.Require()
	serviceName := fmt.Sprintf("go-service-%s", RandStringBytes(12))
	functionName := fmt.Sprintf("go-function-%s", RandStringBytes(8))
	client, err := NewClient(endPoint, "2016-08-15", accessKeyId, accessKeySecret)

	assert.Nil(err)

	defer s.clearService(client, serviceName)

	// CreateService
	_, err2 := client.CreateService(NewCreateServiceInput().
		WithServiceName(serviceName).
		WithDescription("this is a function test for go sdk"))
	assert.Nil(err2)

	// CreateFunction
	createFunctionInput1 := NewCreateFunctionInput(serviceName).WithFunctionName(functionName).
		WithDescription("go sdk test function").
		WithHandler("main.my_handler").WithRuntime("python2.7").
		WithCode(NewCode().
			WithOSSBucketName(codeBucketName).
			WithOSSObjectName("hello_world.zip")).
		WithTimeout(5)
	_, errCreate := client.CreateFunction(createFunctionInput1)
	assert.Nil(errCreate)

	functionName2 := fmt.Sprintf("go-function-%s", RandStringBytes(8))
	_, errReCreate := client.CreateFunction(createFunctionInput1.WithFunctionName(functionName2).WithHandler("hello_world.handler"))
	assert.Nil(errReCreate)
	s.testOssTrigger(client, serviceName, functionName)
	s.testLogTrigger(client, serviceName, functionName)
	s.testHttpTrigger(client, serviceName, functionName2)
	s.testEventBridgeTrigger(client, serviceName, functionName)
}

func (s *FcClientTestSuite) testOssTrigger(client *Client, serviceName, functionName string) {
	assert := s.Require()
	description := "create oss trigger"
	sourceArn := fmt.Sprintf("acs:oss:%s:%s:%s", region, accountID, codeBucketName)
	prefix := fmt.Sprintf("pre%s", RandStringBytes(5))
	suffix := fmt.Sprintf("suf%s", RandStringBytes(5))
	triggerName := "test-oss-trigger"

	createTriggerInput := NewCreateTriggerInput(serviceName, functionName).WithTriggerName(triggerName).
		WithDescription(description).WithInvocationRole(invocationRole).WithTriggerType("oss").
		WithSourceARN(sourceArn).WithTriggerConfig(
		NewOSSTriggerConfig().WithEvents([]string{"oss:ObjectCreated:PostObject"}).WithFilterKeyPrefix(prefix).WithFilterKeySuffix(suffix))

	createTriggerOutput, err := client.CreateTrigger(createTriggerInput)
	assert.Nil(err)
	s.checkTriggerResponse(&createTriggerOutput.triggerMetadata, triggerName, description, "oss", sourceArn, invocationRole)

	getTriggerOutput, err := client.GetTrigger(NewGetTriggerInput(serviceName, functionName, triggerName))
	assert.Nil(err)
	s.checkTriggerResponse(&getTriggerOutput.triggerMetadata, triggerName, description, "oss", sourceArn, invocationRole)

	updateTriggerDesc := "update oss trigger"
	updateTriggerOutput, err := client.UpdateTrigger(NewUpdateTriggerInput(serviceName, functionName, triggerName).WithDescription(updateTriggerDesc).
		WithTriggerConfig(NewOSSTriggerConfig().WithEvents([]string{"oss:ObjectCreated:*"})))
	assert.Nil(err)
	s.checkTriggerResponse(&updateTriggerOutput.triggerMetadata, triggerName, updateTriggerDesc, "oss", sourceArn, invocationRole)
	assert.Equal([]string{"oss:ObjectCreated:*"}, updateTriggerOutput.TriggerConfig.(*OSSTriggerConfig).Events)

	listTriggersOutput, err := client.ListTriggers(NewListTriggersInput(serviceName, functionName))
	assert.Nil(err)
	assert.Equal(len(listTriggersOutput.Triggers), 1)
	_, errReCreate := client.CreateTrigger(createTriggerInput.WithTriggerName(triggerName + "-new").WithTriggerConfig(
		NewOSSTriggerConfig().WithEvents([]string{"oss:ObjectCreated:PostObject"}).WithFilterKeyPrefix(prefix + "-new").WithFilterKeySuffix(suffix + "-new")))
	assert.Nil(errReCreate)
	listTriggersOutput2, err := client.ListTriggers(NewListTriggersInput(serviceName, functionName))
	assert.Nil(err)
	assert.Equal(len(listTriggersOutput2.Triggers), 2)

	_, errDelTrigger := client.DeleteTrigger(NewDeleteTriggerInput(serviceName, functionName, triggerName))
	assert.Nil(errDelTrigger)

	_, errDelTrigger2 := client.DeleteTrigger(NewDeleteTriggerInput(serviceName, functionName, triggerName+"-new"))
	assert.Nil(errDelTrigger2)
}

func (s *FcClientTestSuite) testLogTrigger(client *Client, serviceName, functionName string) {
	assert := s.Require()
	sourceArn := fmt.Sprintf("acs:log:%s:%s:project/%s", region, accountID, logProject)
	description := "create los trigger"
	triggerName := "test-log-trigger"

	logTriggerConfig := NewLogTriggerConfig().WithSourceConfig(NewSourceConfig().WithLogstore(logStore + "_source")).
		WithJobConfig(NewJobConfig().WithMaxRetryTime(10).WithTriggerInterval(60)).
		WithFunctionParameter(map[string]interface{}{}).
		WithLogConfig(NewJobLogConfig().WithProject(logProject).WithLogstore(logStore)).
		WithEnable(false)

	createTriggerInput := NewCreateTriggerInput(serviceName, functionName).WithTriggerName(triggerName).
		WithDescription(description).WithInvocationRole(invocationRole).WithTriggerType("log").
		WithSourceARN(sourceArn).WithTriggerConfig(logTriggerConfig)

	createTriggerOutput, err := client.CreateTrigger(createTriggerInput)
	assert.Nil(err)
	s.checkTriggerResponse(&createTriggerOutput.triggerMetadata, triggerName, description, "log", sourceArn, invocationRole)

	getTriggerOutput, err := client.GetTrigger(NewGetTriggerInput(serviceName, functionName, triggerName))
	assert.Nil(err)
	s.checkTriggerResponse(&getTriggerOutput.triggerMetadata, triggerName, description, "log", sourceArn, invocationRole)

	updateTriggerDesc := "update los trigger"
	updateTriggerOutput, err := client.UpdateTrigger(NewUpdateTriggerInput(serviceName, functionName, triggerName).
		WithDescription(updateTriggerDesc).WithTriggerConfig(logTriggerConfig.WithEnable(true)))
	assert.Nil(err)
	s.checkTriggerResponse(&updateTriggerOutput.triggerMetadata, triggerName, updateTriggerDesc, "log", sourceArn, invocationRole)
	assert.Equal(true, *updateTriggerOutput.TriggerConfig.(*LogTriggerConfig).Enable)

	listTriggersOutput, err := client.ListTriggers(NewListTriggersInput(serviceName, functionName))
	assert.Nil(err)
	assert.Equal(len(listTriggersOutput.Triggers), 1)

	_, errDelTrigger := client.DeleteTrigger(NewDeleteTriggerInput(serviceName, functionName, triggerName))
	assert.Nil(errDelTrigger)
}

func (s *FcClientTestSuite) testHttpTrigger(client *Client, serviceName, functionName string) {
	assert := s.Require()
	sourceArn := "dummy_arn"
	invocationRole := ""
	description := "create http trigger"
	triggerName := "test-http-trigger"

	createTriggerInput := NewCreateTriggerInput(serviceName, functionName).WithTriggerName(triggerName).
		WithDescription(description).WithInvocationRole(invocationRole).WithTriggerType("http").
		WithSourceARN(sourceArn).WithTriggerConfig(
		NewHTTPTriggerConfig().WithAuthType("function").WithMethods("GET", "POST"))

	createTriggerOutput, err := client.CreateTrigger(createTriggerInput)
	assert.Nil(err)
	s.checkTriggerResponse(&createTriggerOutput.triggerMetadata, triggerName, description, "http", sourceArn, invocationRole)

	getTriggerOutput, err := client.GetTrigger(NewGetTriggerInput(serviceName, functionName, triggerName))
	assert.Nil(err)
	s.checkTriggerResponse(&getTriggerOutput.triggerMetadata, triggerName, description, "http", sourceArn, invocationRole)

	updateTriggerDesc := "update http trigger"
	updateTriggerOutput, err := client.UpdateTrigger(NewUpdateTriggerInput(serviceName, functionName, triggerName).
		WithDescription(updateTriggerDesc).WithTriggerConfig(NewHTTPTriggerConfig().WithAuthType("anonymous").
		WithMethods("GET", "POST")))
	assert.Nil(err)
	s.checkTriggerResponse(&updateTriggerOutput.triggerMetadata, triggerName, updateTriggerDesc, "http", sourceArn, invocationRole)
	assert.Equal("anonymous", *updateTriggerOutput.TriggerConfig.(*HTTPTriggerConfig).AuthType)

	listTriggersOutput, err := client.ListTriggers(NewListTriggersInput(serviceName, functionName))
	assert.Nil(err)
	assert.Equal(len(listTriggersOutput.Triggers), 1)

	_, errDelTrigger := client.DeleteTrigger(NewDeleteTriggerInput(serviceName, functionName, triggerName))
	assert.Nil(errDelTrigger)
}

func (s *FcClientTestSuite) testEventBridgeTrigger(client *Client, serviceName, functionName string) {
	generateEventBusName := func(eventSourceType, triggerName string) string {
		if eventSourceType == "Default" {
			return "default"
		}
		return fmt.Sprintf("%s-%s-%s", eventSourceType, functionName, triggerName)
	}
	generateEventRuleName := func(triggerName string) string {
		return fmt.Sprintf("%s-%s-%s", serviceName, functionName, triggerName)
	}
	generateEventRuleArn := func(eventSourceType, triggerName string) string {
		eventBusName := generateEventBusName(eventSourceType, triggerName)
		eventRuleName := generateEventRuleName(triggerName)
		return fmt.Sprintf("acs:eventbridge:%s:%s:eventbus/%s/rule/%s", region, accountID, eventBusName, eventRuleName)
	}
	assert := s.Require()
	description := "create eventbridge trigger"
	updateTriggerDesc := "update eventbridge trigger"
	// Create eventbridge trigger with Default event source type
	defaultTriggerName := "test-eb-trigger-with-default-source"
	defaultEventSourceType := "Default"

	defaultEventSourceConfig := NewEventSourceConfig().WithEventSourceType(defaultEventSourceType)
	createTriggerInput := NewCreateTriggerInput(serviceName, functionName).WithTriggerName(defaultTriggerName).
		WithDescription(description).WithTriggerType(TRIGGER_TYPE_EVENTBRIDGE).
		WithHeader("x-fc-enable-eventbridge-trigger", "enable").WithTriggerConfig(
		NewEventBridgeTriggerConfig().WithTriggerEnable(true).WithAsyncInvocationType(false).
		WithEventRuleFilterPattern("{\"source\": [\"acs.oss\"],\"type\":[\"oss:BucketCreated:PutBucket\"]}").WithEventSourceConfig(defaultEventSourceConfig))
	expectedDefaultSourceArn := generateEventRuleArn(defaultEventSourceType, defaultTriggerName)
	createTriggerOutput, err := client.CreateTrigger(createTriggerInput)
	assert.Nil(err)
	s.checkTriggerResponse(&createTriggerOutput.triggerMetadata, defaultTriggerName, description, TRIGGER_TYPE_EVENTBRIDGE, expectedDefaultSourceArn, "")

	getTriggerOutput, err := client.GetTrigger(NewGetTriggerInput(serviceName, functionName, defaultTriggerName).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(err)
	s.checkTriggerResponse(&getTriggerOutput.triggerMetadata, defaultTriggerName, description, TRIGGER_TYPE_EVENTBRIDGE, expectedDefaultSourceArn, "")

	updateTriggerOutput, err := client.UpdateTrigger(NewUpdateTriggerInput(serviceName, functionName, defaultTriggerName).WithDescription(updateTriggerDesc).
		WithTriggerConfig(NewEventBridgeTriggerConfig().WithAsyncInvocationType(true)).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(err)
	s.checkTriggerResponse(&updateTriggerOutput.triggerMetadata, defaultTriggerName, updateTriggerDesc, TRIGGER_TYPE_EVENTBRIDGE, expectedDefaultSourceArn, "")
	assert.True(*updateTriggerOutput.TriggerConfig.(*EventBridgeTriggerConfig).AsyncInvocationType)

	// Create eventbridge trigger with MNS event source type
	// Creating trigger doesn't rely on the existed resource now, may rely on the existed resource in the future
	mnsTriggerName := "test-eb-trigger-with-mns-source"
	mnsEventSourceType := "MNS"
	sourceMNSParameters := NewSourceMNSParameters().WithRegionId(region).WithQueueName("test-queue").WithIsBase64Decode(true)
	mnsEventSourceParams := NewEventSourceParameters().WithSourceMNSParameters(sourceMNSParameters)

	mnsEventSourceConfig := NewEventSourceConfig().WithEventSourceType(mnsEventSourceType).WithEventSourceParameters(mnsEventSourceParams)
	createTriggerInput = NewCreateTriggerInput(serviceName, functionName).WithTriggerName(mnsTriggerName).
		WithDescription(description).WithTriggerType(TRIGGER_TYPE_EVENTBRIDGE).
		WithHeader("x-fc-enable-eventbridge-trigger", "enable").WithTriggerConfig(
		NewEventBridgeTriggerConfig().WithTriggerEnable(true).WithAsyncInvocationType(false).
			WithEventRuleFilterPattern("{}").WithEventSourceConfig(mnsEventSourceConfig))
	expectedMNSSourceArn := generateEventRuleArn(mnsEventSourceType, mnsTriggerName)

	createTriggerOutput, err = client.CreateTrigger(createTriggerInput)
	assert.Nil(err)
	s.checkTriggerResponse(&createTriggerOutput.triggerMetadata, mnsTriggerName, description, TRIGGER_TYPE_EVENTBRIDGE, expectedMNSSourceArn, "")

	getTriggerOutput, err = client.GetTrigger(NewGetTriggerInput(serviceName, functionName, mnsTriggerName).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(err)
	s.checkTriggerResponse(&getTriggerOutput.triggerMetadata, mnsTriggerName, description, TRIGGER_TYPE_EVENTBRIDGE, expectedMNSSourceArn, "")

	updateTriggerOutput, err = client.UpdateTrigger(NewUpdateTriggerInput(serviceName, functionName, mnsTriggerName).WithDescription(updateTriggerDesc).
		WithTriggerConfig(NewEventBridgeTriggerConfig().WithAsyncInvocationType(true)).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(err)
	s.checkTriggerResponse(&updateTriggerOutput.triggerMetadata, mnsTriggerName, updateTriggerDesc, TRIGGER_TYPE_EVENTBRIDGE, expectedMNSSourceArn, "")
	assert.True(*updateTriggerOutput.TriggerConfig.(*EventBridgeTriggerConfig).AsyncInvocationType)

	// Create eventbridge trigger with RocketMQ event source type
	// Creating trigger doesn't rely on the existed resource now, may rely on the existed resource in the future
	rocketMQTriggerName := "test-eb-trigger-with-rocketmq-source"
	rocketMQEventSourceType := "RocketMQ"
	sourceRocketMQParameters := NewSourceRocketMQParameters().WithRegionId(region).WithGroupID("test-group").
			WithInstanceId("test-instance").WithTopic("test-topic").WithTimestamp(1636597951984)
	rocketMQEventSourceParams := NewEventSourceParameters().WithSourceRocketMQParameters(sourceRocketMQParameters)

	rocketMQEventSourceConfig := NewEventSourceConfig().WithEventSourceType(rocketMQEventSourceType).WithEventSourceParameters(rocketMQEventSourceParams)
	createTriggerInput = NewCreateTriggerInput(serviceName, functionName).WithTriggerName(rocketMQTriggerName).
		WithDescription(description).WithTriggerType(TRIGGER_TYPE_EVENTBRIDGE).
		WithHeader("x-fc-enable-eventbridge-trigger", "enable").WithTriggerConfig(
		NewEventBridgeTriggerConfig().WithTriggerEnable(true).WithAsyncInvocationType(false).
			WithEventRuleFilterPattern("{}").WithEventSourceConfig(rocketMQEventSourceConfig))
	expectedRocketMQSourceArn := generateEventRuleArn(rocketMQEventSourceType, rocketMQTriggerName)

	createTriggerOutput, err = client.CreateTrigger(createTriggerInput)
	assert.Nil(err)
	s.checkTriggerResponse(&createTriggerOutput.triggerMetadata, rocketMQTriggerName, description, TRIGGER_TYPE_EVENTBRIDGE, expectedRocketMQSourceArn, "")

	getTriggerOutput, err = client.GetTrigger(NewGetTriggerInput(serviceName, functionName, rocketMQTriggerName).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(err)
	s.checkTriggerResponse(&getTriggerOutput.triggerMetadata, rocketMQTriggerName, description, TRIGGER_TYPE_EVENTBRIDGE, expectedRocketMQSourceArn, "")

	updateTriggerOutput, err = client.UpdateTrigger(NewUpdateTriggerInput(serviceName, functionName, rocketMQTriggerName).WithDescription(updateTriggerDesc).
		WithTriggerConfig(NewEventBridgeTriggerConfig().WithAsyncInvocationType(true)).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(err)
	s.checkTriggerResponse(&updateTriggerOutput.triggerMetadata, rocketMQTriggerName, updateTriggerDesc, TRIGGER_TYPE_EVENTBRIDGE, expectedRocketMQSourceArn, "")
	assert.True(*updateTriggerOutput.TriggerConfig.(*EventBridgeTriggerConfig).AsyncInvocationType)

	// Create eventbridge trigger with RabbitMQ event source type
	// Creating trigger doesn't rely on the existed resource now, may rely on the existed resource in the future
	rabbitMQTriggerName := "test-eb-trigger-with-rabbitmq-source"
	rabbitMQEventSourceType := "RabbitMQ"
	sourceRabbitMQParameters := NewSourceRabbitMQParameters().WithRegionId(region).WithInstanceId("test-instance").
		WithQueueName("test-queue").WithVirtualHostName("test-virtual")
	rabbitMQEventSourceParams := NewEventSourceParameters().WithSourceRabbitMQParameters(sourceRabbitMQParameters)

	rabbitMQEventSourceConfig := NewEventSourceConfig().WithEventSourceType(rabbitMQEventSourceType).WithEventSourceParameters(rabbitMQEventSourceParams)
	createTriggerInput = NewCreateTriggerInput(serviceName, functionName).WithTriggerName(rabbitMQTriggerName).
		WithDescription(description).WithTriggerType(TRIGGER_TYPE_EVENTBRIDGE).
		WithHeader("x-fc-enable-eventbridge-trigger", "enable").WithTriggerConfig(
		NewEventBridgeTriggerConfig().WithTriggerEnable(true).WithAsyncInvocationType(false).
			WithEventRuleFilterPattern("{}").WithEventSourceConfig(rabbitMQEventSourceConfig))
	expectedRabbitMQSourceArn := generateEventRuleArn(rabbitMQEventSourceType, rabbitMQTriggerName)

	createTriggerOutput, err = client.CreateTrigger(createTriggerInput)
	assert.Nil(err)
	s.checkTriggerResponse(&createTriggerOutput.triggerMetadata, rabbitMQTriggerName, description, TRIGGER_TYPE_EVENTBRIDGE, expectedRabbitMQSourceArn, "")

	getTriggerOutput, err = client.GetTrigger(NewGetTriggerInput(serviceName, functionName, rabbitMQTriggerName).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(err)
	s.checkTriggerResponse(&getTriggerOutput.triggerMetadata, rabbitMQTriggerName, description, TRIGGER_TYPE_EVENTBRIDGE, expectedRabbitMQSourceArn, "")

	updateTriggerOutput, err = client.UpdateTrigger(NewUpdateTriggerInput(serviceName, functionName, rabbitMQTriggerName).WithDescription(updateTriggerDesc).
		WithTriggerConfig(NewEventBridgeTriggerConfig().WithAsyncInvocationType(true)).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(err)
	s.checkTriggerResponse(&updateTriggerOutput.triggerMetadata, rabbitMQTriggerName, updateTriggerDesc, TRIGGER_TYPE_EVENTBRIDGE, expectedRabbitMQSourceArn, "")
	assert.True(*updateTriggerOutput.TriggerConfig.(*EventBridgeTriggerConfig).AsyncInvocationType)


	listTriggersOutput, err := client.ListTriggers(NewListTriggersInput(serviceName, functionName).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(err)
	assert.Equal(len(listTriggersOutput.Triggers), 4)

	_, errDelDefaultTrigger := client.DeleteTrigger(NewDeleteTriggerInput(serviceName, functionName, defaultTriggerName).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(errDelDefaultTrigger)

	_, errDelMNSTrigger := client.DeleteTrigger(NewDeleteTriggerInput(serviceName, functionName, mnsTriggerName).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(errDelMNSTrigger)

	_, errDelRocketMQTrigger := client.DeleteTrigger(NewDeleteTriggerInput(serviceName, functionName, rocketMQTriggerName).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(errDelRocketMQTrigger)

	_, errDelRabbitMQTrigger := client.DeleteTrigger(NewDeleteTriggerInput(serviceName, functionName, rabbitMQTriggerName).WithHeader("x-fc-enable-eventbridge-trigger", "enable"))
	assert.Nil(errDelRabbitMQTrigger)
}

func (s *FcClientTestSuite) checkTriggerResponse(triggerResp *triggerMetadata, triggerName, description, triggerType, sourceArn, invocationRole string) {
	assert := s.Require()
	assert.Equal(*triggerResp.TriggerName, triggerName)
	assert.Equal(*triggerResp.Description, description)
	assert.Equal(*triggerResp.TriggerType, triggerType)
	if triggerType != "http" {
		assert.Equal(*triggerResp.SourceARN, sourceArn)
	} else {
		assert.Nil(triggerResp.SourceARN)
	}
	if triggerType != TRIGGER_TYPE_EVENTBRIDGE {
		assert.Equal(*triggerResp.InvocationRole, invocationRole)
	}
	assert.NotNil(*triggerResp.CreatedTime)
	assert.NotNil(*triggerResp.LastModifiedTime)
}

func (s *FcClientTestSuite) clearService(client *Client, serviceName string) {
	assert := s.Require()
	// DeleteFunction
	listFunctionsOutput, err := client.ListFunctions(NewListFunctionsInput(serviceName).WithLimit(10))
	assert.Nil(err)
	for _, fuc := range listFunctionsOutput.Functions {
		functionName := *fuc.FunctionName
		listTriggersOutput, err := client.ListTriggers(NewListTriggersInput(serviceName, functionName))
		assert.Nil(err)
		for _, trigger := range listTriggersOutput.Triggers {
			_, errDelTrigger := client.DeleteTrigger(NewDeleteTriggerInput(serviceName, functionName, *trigger.TriggerName))
			assert.Nil(errDelTrigger)
		}

		_, errDelFunc := client.DeleteFunction(NewDeleteFunctionInput(serviceName, functionName))
		assert.Nil(errDelFunc)
	}
	// DeleteService and clear all tags
	resArn := fmt.Sprintf("services/%s", serviceName)
	client.UnTagResource(NewUnTagResourceInput(resArn).WithTagKeys([]string{}).WithAll(true))
	_, errDelService := client.DeleteService(NewDeleteServiceInput(serviceName))
	assert.Nil(errDelService)
}
