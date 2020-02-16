package main
import (
	"net/http"
	"os/exec"
	"io/ioutil"
	"encoding/json"
	"encoding/base64"
	"context"
	"log"
	"os"
	"path/filepath"
	"time"
	"github.com/gin-gonic/gin"
)
type result struct {
	Code	int		`json:"code"`
	Message	string	`json:"message"`
	File	string	`json:"file"`
}
type requestBody struct {
	Database		string	`json:"database"`
	Username		string	`json:"username"`
	Password		string	`json:"password"`
	File			string	`json:"file"`
	FileShortName	string	`json:"fileShortName"`
	DocumentType	string	`json:"documentType"`
}
func serve(closesignal chan int) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.POST("/importXML", importXML)
	srv := &http.Server{
		Addr:    ":7005",
		Handler: r,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	<-closesignal
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
	}
	select {
	case <-ctx.Done():
		srv.Close()
	}
}
func importXML(c *gin.Context) {
	t1 := time.Now()
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	logFilename := filepath.Join(dir, "mikroConsoleApi.log")
	exeFilename := filepath.Join(dir, "MikroConsoleApp.exe")
	f, _ := os.OpenFile(logFilename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	log.SetOutput(f)
	defer f.Close()
	log.Printf("ImportXML command called.\n")
	var request requestBody  
	err := c.BindJSON(&request)
	if err!=nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to parse request body.",
		})
	}else
	{
		inputFilename := filepath.Join(dir,request.FileShortName+".xml")
		fileInBytes,err :=base64.StdEncoding.DecodeString(request.File)
		ioutil.WriteFile(inputFilename,fileInBytes,0666)
		ctx, _ := context.WithTimeout(context.Background(), 25*time.Second)
		cmd := exec.CommandContext(ctx,exeFilename,request.Database,request.Username,request.Password,request.FileShortName,request.DocumentType)
		cmd.Dir=dir
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Printf("StderrPipe():%s\n",err.Error())
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("StdoutPipe():%s\n",err.Error())
		}
		if err := cmd.Start(); err != nil {
			log.Printf("Command.Start():%s\n",err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Command could not start."+err.Error(),
			})
		}else {
			var cmdResult result
			jsonErr := json.NewDecoder(stdout).Decode(&cmdResult)
			notifier:=false
			slurp, _ := ioutil.ReadAll(stderr)
			if len(slurp) > 0 {
				log.Printf("Execute error.%s\n",slurp)
				c.JSON(http.StatusNotImplemented, gin.H{
					"message": slurp,
				})
			}else{
				if jsonErr != nil {
					log.Printf("Json.Decode(): %s\n",jsonErr.Error())
					notifier=true 
				}else{
					c.JSON(cmdResult.Code, gin.H{
						"message":cmdResult.Message,
						"file":cmdResult.File,
					})
				}
			}
			if err := cmd.Wait(); err != nil {
				log.Printf("Non-zero exit code.%s\n",err.Error())
			}
			err = ctx.Err()
			if err == context.DeadlineExceeded {
				if notifier {
					c.JSON(http.StatusInternalServerError, gin.H{
						"message":"Command time out.",
					})
					notifier=false  
				}
				log.Printf("Context error.Command time out.\n")
			}else if err!=nil {
				log.Printf("Context error.%s\n",err.Error())
			}
			if notifier{
				c.JSON(http.StatusInternalServerError, gin.H{
					"message":"Unable to parse command response.",
				})
			}
		}
		t2:=time.Now()
		diff:=t2.Sub(t1).Milliseconds()
		log.Printf("Function execution %d miliseconds long.\n",diff)
	}
}