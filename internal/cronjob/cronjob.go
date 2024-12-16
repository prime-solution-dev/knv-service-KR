package cronjob

import (
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

var (
	c           *cron.Cron                      // ตัวแปร cron global
	mu          sync.RWMutex                    // ใช้เพื่อจัดการ thread-safe เมื่อทำงานกับ cron
	jobCronMap  = make(map[string]cron.EntryID) // ใช้เก็บ cronID ของแต่ละ job
	jobState    = make(map[string]bool)         // ใช้เก็บสถานะของแต่ละ job (true = running)
	jobRegistry = make(map[string]JobDetail)    // ใช้เก็บฟังก์ชันและ expression ของแต่ละ job
)

type JobDetail struct {
	JobFunc        func() // ฟังก์ชันของงาน
	CronExpression string // Expression ของงาน
}

func AutoStartCronJobs() {

	if c == nil {
		createCron()
	}

	for jobName := range jobRegistry {
		startCron(jobName)
	}

	c.Start()
}

func StartJob(jobName string) {
	startCron(jobName)
}

func StopJob(jobName string) {
	stopCron(jobName)
}

func RefreshJobs() {
	for jobName := range jobCronMap {
		stopCron(jobName)
	}

	for jobName := range jobRegistry {
		startCron(jobName)
	}
}

func RegisterJob(jobName string, jobFunc func(), cronExpression string) {
	registerCron(jobName, JobDetail{
		JobFunc:        jobFunc,
		CronExpression: cronExpression,
	})
}

func createCron() {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		c = cron.New()
	}
}

func startCron(jobName string) {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		log.Println("Cron instance is not initialized.")
		return
	}

	if jobState[jobName] {
		log.Printf("Job %s is already running, skipping this run.\n", jobName)
		return
	}

	jobDetail, exists := jobRegistry[jobName]
	if !exists {
		log.Printf("Job %s does not exist in jobRegistry.\n", jobName)
		return
	}

	cronID, err := c.AddFunc(jobDetail.CronExpression, func() {
		mu.Lock()
		if jobState[jobName] {
			mu.Unlock()
			return
		}

		jobState[jobName] = true
		mu.Unlock()

		jobDetail.JobFunc()

		mu.Lock()
		jobState[jobName] = false
		mu.Unlock()
	})
	if err != nil {
		log.Fatalf("Error starting job %s: %v\n", jobName, err)
	}

	jobCronMap[jobName] = cronID
}

func stopCron(jobName string) {
	mu.Lock()
	defer mu.Unlock()

	if !jobState[jobName] {
		return
	}

	c.Remove(jobCronMap[jobName])
	delete(jobCronMap, jobName)
	jobState[jobName] = false
}

func registerCron(jobName string, jd JobDetail) {
	mu.Lock()
	defer mu.Unlock()

	jobRegistry[jobName] = jd
}
