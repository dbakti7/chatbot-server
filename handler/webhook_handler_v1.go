package handler
import (
    "net/http"
    "io/ioutil"
    "fmt"
    "encoding/json"
    "../utils"
    "../course"
    "time"
    "../storage"
    "github.com/tidwall/gjson"
    "strconv"
    "sort"
    "strings"
    "os"
)

func WebhookHandlerV1(rw http.ResponseWriter, req *http.Request) {
    defer utils.TimeFunction(time.Now(), "w")
    db, err := storage.NewDB("test.sqlite3")
    body, err := ioutil.ReadAll(req.Body)

    if(err != nil) {
        panic(err)
    }
    //fmt.Println(string(body[:]))
    fullJSON := string(body[:])
    
    // Get session ID
    sessionID := gjson.Get(fullJSON, "sessionId")

    // Get query parameters, sort them
    paramsJSON := gjson.Get(fullJSON, "result.parameters")
    params := make([]string, 0)
    var number string
    hasNumber := false
    paramsJSON.ForEach(func(key, value gjson.Result) bool {
        for _, elem := range value.Array() {
            if(elem.String() != "") {
                // TODO: check DialogFlow system number entity instead
                if _, err := strconv.Atoi(elem.String()); err == nil {
                    number = elem.String()
                    hasNumber = true
                } else {
                    params = append(params, strings.ToLower(elem.String()))
                }
            }
        }
        return true
    })
    sort.Strings(params)
    if hasNumber {
        params[0] = params[0] + number
    }

    //originalRequest := gjson.Get(string(body[:]), "originalRequest.data.message.text")

    // get original request text, intent and contexts
    originalRequest := gjson.Get(fullJSON, "result.resolvedQuery")
    intent := gjson.Get(fullJSON, "result.metadata.intentName")
    contextsJSON := gjson.Get(fullJSON, "result.contexts")
    contexts := make([]string, 0)
    for _, elem := range contextsJSON.Array() {
        contexts = append(contexts, gjson.Get(elem.String(), "name").String())
    }

    // preparing response
    var resultMap map[string]interface{}
    resultMap = make(map[string]interface{})

    // for debugging
    // fmt.Println(originalRequest)
    // fmt.Println(params)
    // fmt.Println(intent)

    // TODO: fill with proper values here
    resultMap["displayText"] = "Test Response"
    resultMap["speech"] = "Response not found"
    resultMap["data"] = ""
    resultMap["contextOut"] = []string{}
    resultMap["source"] = "Hello"

    // file logging
    f, err := os.OpenFile("log-alpha2.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if(err != nil) {
        fmt.Printf("error %v", err)
    }
    defer f.Close()
    f.WriteString("Query from: " + sessionID.String() + "\r\n")
    f.WriteString(originalRequest.String() + "\r\n")
    f.WriteString("----------\r\n")
    f.WriteString("Intent: \r\n" + intent.String() + "\r\n")
    f.WriteString("----------\r\n")

    if(strings.Compare(intent.String(), "Course") == 0) {
        // course related
        for _, elem := range params {
            courseCode := course.ParseCourseCode(elem)
            if(courseCode != "") {
                course, _ := db.GetCourseByCode(courseCode)
                for _, field := range params {
                    if(field == "course code") {
                        resultMap["speech"] = course.Code
                    } else if(field == "course name") {
                        resultMap["speech"] = course.Name
                    } else if(field == "au") {
                        resultMap["speech"] = course.AU
                    } else if(field == "course description") {
                        resultMap["speech"] = course.Description
                    } else if(field == "prereq") {
                        resultMap["speech"] = course.PreReq
                    }
                }
            }
        }
    } else if strings.Compare(intent.String(), "location") == 0 {
        // location queries
        resultMap["speech"] = "Please refer to http://maps.ntu.edu.sg/maps#q:" +
        strings.Replace(params[0], " ", "%20", -1) + "\r\n"
    } else {
        // other queries
        all, _ := db.ListRecordsByIntent(intent.String())
        maxMatchParams := 0
        for _, elem := range all {
            dbValue := strings.Split(elem.Params, ",")
            for index, _ := range dbValue {
                dbValue[index] = strings.TrimSpace(dbValue[index])
                dbValue[index] = strings.ToLower(dbValue[index])
            }
            sort.Strings(dbValue)
            currentMatchParams := 0
            for _, param := range dbValue {
                if utils.Contains(params, param) {
                    currentMatchParams += 1
                }
                // default response, if any
                if maxMatchParams == 0 && param == "default" {
                    resultMap["speech"] = elem.Response
                }
            }
            if(currentMatchParams > maxMatchParams) {
                maxMatchParams = currentMatchParams
                resultMap["speech"] = elem.Response
            }
        }
    }

    // default fallback: direct to google search, get the first result
    if strings.Compare(resultMap["speech"].(string), "Response not found") == 0 {
        resp, err := http.Get("https://www.googleapis.com/customsearch/v1?q=" + 
            "ntu+singapore+" + strings.Replace(originalRequest.String(), " ", "+", -1) + "&cx=000348109821987500770%3Ar1ufthpxqxg&key=AIzaSyDW0l64m7xweAo28Z_q3yAskU_d5fbevGw")
        if err != nil {
            // handle error
        }
        defer resp.Body.Close()
        body, err := ioutil.ReadAll(resp.Body)
        results := gjson.Get(string(body[:]), "items")
        for _, elem := range results.Array() {
            link := gjson.Get(elem.String(), "link").String()
            resultMap["speech"] = "You can find out more about it at " + link + "\r\n"
            break
        }
    }

    f.WriteString("Response:\r\n")
    f.WriteString(strings.Replace(resultMap["speech"].(string), "\n", "\r\n", -1) + "\r\n")
    f.WriteString("----------\r\n")

    resultJson, _ := json.Marshal(resultMap)
    
    rw.Header().Set("Content-Type", "application/json")
        
    rw.Write(resultJson)
}