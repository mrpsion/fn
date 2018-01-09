package datastoreutil

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/fnproject/fn/api/models"
)

// NewValidator returns a models.Datastore which validates certain arguments before delegating to ds.
func NewValidator(ds models.Datastore) models.Datastore {
	return &validator{ds}
}

type validator struct {
	models.Datastore
}

func checkApp(app *models.App) error {
	if app == nil {
		return models.ErrDatastoreEmptyApp
	}
	if app.Name == "" {
		return models.ErrDatastoreEmptyAppName
	}
	return nil
}

func checkRoute(route *models.Route) error {
	if route == nil {
		return models.ErrDatastoreEmptyRoute
	}
	if route.AppID == "" {
		return models.ErrDatastoreEmptyApp
	}
	if route.Path == "" {
		return models.ErrDatastoreEmptyRoutePath
	}
	return nil
}

// name will never be empty.
func (v *validator) GetApp(ctx context.Context, app *models.App) (*models.App, error) {
	err := checkApp(app)
	if err != nil {
		return nil, err
	}

	return v.Datastore.GetApp(ctx, app)
}

func (v *validator) GetApps(ctx context.Context, appFilter *models.AppFilter) ([]*models.App, error) {
	return v.Datastore.GetApps(ctx, appFilter)
}

// app and app.Name will never be nil/empty.
func (v *validator) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	err := checkApp(app)
	if err != nil {
		return nil, err
	}

	return v.Datastore.InsertApp(ctx, app)
}

// app and app.Name will never be nil/empty.
func (v *validator) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	err := checkApp(app)
	if err != nil {
		return nil, err
	}

	return v.Datastore.UpdateApp(ctx, app)
}

// name will never be empty.
func (v *validator) RemoveApp(ctx context.Context, app *models.App) error {
	err := checkApp(app)
	if err != nil {
		return err
	}

	return v.Datastore.RemoveApp(ctx, app)
}

// appName and routePath will never be empty.
func (v *validator) GetRoute(ctx context.Context, app *models.App, routePath string) (*models.Route, error) {
	err := checkApp(app)
	if err != nil {
		return nil, err
	}

	if routePath == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	return v.Datastore.GetRoute(ctx, app, routePath)
}

// appName will never be empty
func (v *validator) GetRoutesByApp(ctx context.Context, app *models.App, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	err = checkApp(app)
	if err != nil {
		return nil, err
	}

	return v.Datastore.GetRoutesByApp(ctx, app, routeFilter)
}

// route will never be nil and route's AppName and Path will never be empty.
func (v *validator) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	err := checkRoute(route)
	if err != nil {
		return nil, err
	}

	return v.Datastore.InsertRoute(ctx, route)
}

// route will never be nil and route's AppName and Path will never be empty.
func (v *validator) UpdateRoute(ctx context.Context, newroute *models.Route) (*models.Route, error) {
	err := checkRoute(newroute)
	if err != nil {
		return nil, err
	}

	return v.Datastore.UpdateRoute(ctx, newroute)
}

// appName and routePath will never be empty.
func (v *validator) RemoveRoute(ctx context.Context, app *models.App, routePath string) error {
	err := checkApp(app)
	if err != nil {
		return err
	}

	if routePath == "" {
		return models.ErrDatastoreEmptyRoutePath
	}

	return v.Datastore.RemoveRoute(ctx, app, routePath)
}

// callID will never be empty.
func (v *validator) GetCall(ctx context.Context, appName, callID string) (*models.Call, error) {
	if callID == "" {
		return nil, models.ErrDatastoreEmptyCallID
	}
	return v.Datastore.GetCall(ctx, appName, callID)
}

// GetDatabase returns the underlying sqlx database implementation
func (v *validator) GetDatabase() *sqlx.DB {
	return v.Datastore.GetDatabase()
}
