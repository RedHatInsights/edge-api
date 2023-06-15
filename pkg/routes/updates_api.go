package routes

// @Summary      Gets all device updates
// @Tags         Updates
// @Accept       json
// @Produce      json
// @Param        id         path  int  true  "Account ID"
// @Param        sort_by    query enumstring false "One of created_at, updated_at"
// @Param        created_at query string     false "An ISO-8601 date in YYYY-MM-DD format"
// @Param        updated_at query string     false "An ISO-8601 date in YYYY-MM-DD format"
// @Param        status     query string     false "Filter by status"
// @Param        limit      query int        false "The number of updates to return" default(30)
// @Param        offset     query int        false "The offset to begin returning updates"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /updates [get]
type GetUpdatesAllDevicesEndpoint struct{}

// @Summary      Return list of available updates for a device
// @Tags         Updates
// @Accept       json
// @Produce      json
// @param        DeviceUUID path  string    true  "The device UUID"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} errors.BadRequest
// @Router       /updates/device/{DeviceUUID} [get]
type GetUpdatesDeviceByIDEndpoint struct{}

// @Summary      Return image running on device
// @Tags         Updates
// @Accept       json
// @Produce      json
// @param        DeviceUUID path  string    true  "The device UUID"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /updates/device/{DeviceUUID}/image [get]
type GetUpdatesDeviceByIDImageEndpoint struct{}

// @Summary      Return list of available updates for a device
// @Tags         Updates
// @Accept       json
// @Produce      json
// @param        DeviceUUID path  string     true  "The device UUID"
// @param        latest     query enumstring false "One of: latest, all"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} models.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /updates/device/{DeviceUUID}/updates [get]
type GetUpdatesDeviceByIDUpdatesEndpoint struct{}

// @Summary      Gets a single requested update
// @Tags         Updates
// @Accept       json
// @Produce      json
// @Param        updateID   path  int        true  "A unique ID to identify the update"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} models.BadRequest
// @Failure      404 {object} errors.NotFound
// @Failure      500 {object} errors.InternalServerError
// @Router       /updates/{UpdateID} [get]
type GetUpdatesSingleUpdateByIDEndpoint struct{}

// @Summary      Returns the playbook for a update transaction
// @Tags         Updates
// @Accept       json
// @Produce      json
// @Param        updateID   path  int        true  "A unique ID to identify the update"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} models.BadRequest
// @Failure      404 {object} errors.NotFound
// @Failure      500 {object} errors.InternalServerError
// @Router       /updates/{updateID}/update-playbook.yml [get]
type GetUpdatesSingleUpdateByIDPlaybookEndpoint struct{}

// @Summary      Executes a device update
// @Tags         Updates
// @Accept       json
// @Produce      json
// @Param        CommitID    body  int         false "A unique ID to identify a commit" minimum(0)
// @Param        DevicesUUID body  []string    false "An array of unique device IDs"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} models.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /updates [post]
type PostUpdatesDeviceEndpoint struct{}

// @Summary      Validates if it's possible to update the images selection
// @Tags         Updates
// @Accept       json
// @Produce      json
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} models.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /updates/validate [post]
type PostUpdatesValidateEndpoint struct{}
