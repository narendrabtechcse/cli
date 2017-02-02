package v2action

import (
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/api/cloudcontroller"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2"
)

type ApplicationInstanceState ccv2.ApplicationInstanceState

type ApplicationInstance struct {
	// CPU is the instance's CPU utilization percentage.
	CPU float64

	// Details are arbitrary information about the instance.
	Details string

	// Disk is the instance's disk usage in bytes.
	Disk int

	// DiskQuota is the instance's allowed disk usage in bytes.
	DiskQuota int

	// ID is the instance ID.
	ID int

	// Memory is the instance's memory usage in bytes.
	Memory int

	// MemoryQuota is the instance's allowed memory usage in bytes.
	MemoryQuota int

	// Since is the Unix time stamp that represents the time the instance was
	// created.
	Since float64

	// State is the instance's state.
	State ApplicationInstanceState
}

// NewApplicationInstance returns a pointer to a new ApplicationInstance.
func NewApplicationInstance(id int) *ApplicationInstance {
	return &ApplicationInstance{ID: id}
}

func (instance ApplicationInstance) TimeSinceCreation() time.Time {
	return time.Unix(int64(instance.Since), 0)
}

func (instance *ApplicationInstance) setInstance(ccAppInstance ccv2.ApplicationInstance) {
	instance.Details = ccAppInstance.Details
	instance.Since = ccAppInstance.Since
	instance.State = ApplicationInstanceState(ccAppInstance.State)
}

func (instance *ApplicationInstance) setStats(ccAppStats ccv2.ApplicationInstanceStatus) {
	instance.CPU = ccAppStats.CPU
	instance.Disk = ccAppStats.Disk
	instance.DiskQuota = ccAppStats.DiskQuota
	instance.Memory = ccAppStats.Memory
	instance.MemoryQuota = ccAppStats.MemoryQuota
}

func (instance *ApplicationInstance) incomplete() {
	instance.Details = strings.TrimSpace(fmt.Sprintf("%s (%s)", instance.Details, "Unable to retrieve information"))
}

// ApplicationInstancesNotFoundError is returned when a requested application is not
// found.
type ApplicationInstancesNotFoundError struct {
	ApplicationGUID string
}

func (e ApplicationInstancesNotFoundError) Error() string {
	return fmt.Sprintf("Application instances '%s' not found.", e.ApplicationGUID)
}

func (actor Actor) GetApplicationInstancesByApplication(guid string) ([]ApplicationInstance, Warnings, error) {
	var allWarnings Warnings

	ccAppInstanceStatuses, warnings, err := actor.CloudControllerClient.GetApplicationInstanceStatusesByApplication(guid)
	allWarnings = append(allWarnings, warnings...)

	switch err.(type) {
	case cloudcontroller.ResourceNotFoundError:
		return nil, allWarnings, ApplicationInstancesNotFoundError{ApplicationGUID: guid}
	case nil:
		// continue
	default:
		return nil, allWarnings, err
	}

	ccAppInstances, warnings, err := actor.CloudControllerClient.GetApplicationInstancesByApplication(guid)
	allWarnings = append(allWarnings, warnings...)

	if err != nil {
		return nil, allWarnings, err
	}

	appInstances := []ApplicationInstance{}

	seenStatuses := make(map[int]bool, len(ccAppInstanceStatuses))

	for id, appInstanceStatus := range ccAppInstanceStatuses {
		seenStatuses[id] = true

		appInstance := NewApplicationInstance(id)
		appInstance.setStats(appInstanceStatus)

		if ccAppInstance, found := ccAppInstances[id]; found {
			appInstance.setInstance(ccAppInstance)
		} else {
			appInstance.incomplete()
		}

		appInstances = append(appInstances, *appInstance)
	}

	// add instances that are missing stats
	for index, instance := range ccAppInstances {
		if _, found := seenStatuses[index]; !found {
			appInstance := NewApplicationInstance(index)
			appInstance.setInstance(instance)
			appInstance.incomplete()

			appInstances = append(appInstances, *appInstance)
		}
	}

	return appInstances, allWarnings, err

	// return sortAppInstancesByID(appInstances), allWarnings, err
}

// func sortAppInstancesByID(instances []ApplicationInstance) []ApplicationInstance {
// 	instancesMap := map[int]ApplicationInstance{}
// 	var ids []int

// 	for _, instance := range instances {
// 		ids = append(ids, instance.ID)
// 		instancesMap[instance.ID] = instance
// 	}

// 	sort.Ints(ids)

// 	var sortedInstances []ApplicationInstance

// 	for id := range ids {
// 		sortedInstances = append(sortedInstances, instancesMap[id])
// 	}

// 	return sortedInstances
// }

// func (client *Client) sortedInstanceKeys(instances map[string]ApplicationInstanceStatus) ([]int, error) {
// 	var keys []int
// 	for key, _ := range instances {
// 		id, err := strconv.Atoi(key)
// 		if err != nil {
// 			return nil, err
// 		}
// 		keys = append(keys, id)
// 	}
// 	sort.Ints(keys)

// 	return keys, nil
// }
