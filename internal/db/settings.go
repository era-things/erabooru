package db

import (
	"context"
	"errors"
	"strings"

	"era/booru/ent"
	"era/booru/ent/hiddentagfilter"
	"era/booru/ent/setting"
)

const (
	hiddenTagDefaultValue           = ""
	settingKeyActiveHiddenTagFilter = "active_hidden_tag_filter"
)

// EnsureHiddenTagDefaults makes sure that the default hidden tag filter exists and
// that the active setting points to a valid filter.
func EnsureHiddenTagDefaults(ctx context.Context, client *ent.Client) (err error) {
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	defaultFilter, err := tx.HiddenTagFilter.Query().
		Where(hiddentagfilter.ValueEQ(hiddenTagDefaultValue)).
		Only(ctx)
	if ent.IsNotFound(err) {
		defaultFilter, err = tx.HiddenTagFilter.Create().
			SetValue(hiddenTagDefaultValue).
			Save(ctx)
	}
	if err != nil {
		return err
	}

	currentSetting, err := tx.Setting.Query().
		Where(setting.KeyEQ(settingKeyActiveHiddenTagFilter)).
		Only(ctx)
	switch {
	case ent.IsNotFound(err):
		if _, err = tx.Setting.Create().
			SetKey(settingKeyActiveHiddenTagFilter).
			SetValue(defaultFilter.Value).
			Save(ctx); err != nil {
			if ent.IsConstraintError(err) {
				if err := tx.Setting.Update().
					Where(setting.KeyEQ(settingKeyActiveHiddenTagFilter)).
					SetValue(defaultFilter.Value).
					Exec(ctx); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	case err != nil:
		return err
	default:
		if currentSetting.Value != hiddenTagDefaultValue {
			exists, err := tx.HiddenTagFilter.Query().
				Where(hiddentagfilter.ValueEQ(currentSetting.Value)).
				Exist(ctx)
			if err != nil {
				return err
			}
			if !exists {
				if err := tx.Setting.Update().
					Where(setting.KeyEQ(settingKeyActiveHiddenTagFilter)).
					SetValue(defaultFilter.Value).
					Exec(ctx); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// ListHiddenTagFilters returns the available hidden tag filters ordered by creation time.
func ListHiddenTagFilters(ctx context.Context, client *ent.Client) ([]*ent.HiddenTagFilter, error) {
	return client.HiddenTagFilter.Query().
		Order(hiddentagfilter.ByCreatedAt()).
		All(ctx)
}

// CreateHiddenTagFilter inserts a new hidden tag filter using the provided expression.
func CreateHiddenTagFilter(ctx context.Context, client *ent.Client, value string) (*ent.HiddenTagFilter, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, errors.New("hidden tag filter cannot be empty")
	}
	return client.HiddenTagFilter.Create().
		SetValue(trimmed).
		Save(ctx)
}

// DeleteHiddenTagFilter removes a hidden tag filter by ID. The default filter cannot be deleted.
func DeleteHiddenTagFilter(ctx context.Context, client *ent.Client, id int) error {
	filter, err := client.HiddenTagFilter.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if filter.Value == hiddenTagDefaultValue {
		return errors.New("cannot delete the default hidden tag filter")
	}
	if err := client.HiddenTagFilter.DeleteOneID(id).Exec(ctx); err != nil {
		return err
	}

	activeValue, err := ActiveHiddenTagFilterValue(ctx, client)
	if err != nil {
		return err
	}
	if activeValue == filter.Value {
		return SetActiveHiddenTagFilterValue(ctx, client, hiddenTagDefaultValue)
	}
	return nil
}

// SetActiveHiddenTagFilter stores the provided filter ID as the active one.
func SetActiveHiddenTagFilter(ctx context.Context, client *ent.Client, id int) error {
	filter, err := client.HiddenTagFilter.Get(ctx, id)
	if err != nil {
		return err
	}
	return SetActiveHiddenTagFilterValue(ctx, client, filter.Value)
}

// SetActiveHiddenTagFilterValue updates the active hidden tag filter to the provided value.
func SetActiveHiddenTagFilterValue(ctx context.Context, client *ent.Client, value string) error {
	_, err := client.Setting.Create().
		SetKey(settingKeyActiveHiddenTagFilter).
		SetValue(value).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return client.Setting.Update().
				Where(setting.KeyEQ(settingKeyActiveHiddenTagFilter)).
				SetValue(value).
				Exec(ctx)
		}
		return err
	}
	return nil
}

// ActiveHiddenTagFilterValue returns the expression for the currently active filter.
func ActiveHiddenTagFilterValue(ctx context.Context, client *ent.Client) (string, error) {
	settingRow, err := client.Setting.Query().
		Where(setting.KeyEQ(settingKeyActiveHiddenTagFilter)).
		Only(ctx)
	if ent.IsNotFound(err) {
		if err := EnsureHiddenTagDefaults(ctx, client); err != nil {
			return hiddenTagDefaultValue, err
		}
		settingRow, err = client.Setting.Query().
			Where(setting.KeyEQ(settingKeyActiveHiddenTagFilter)).
			Only(ctx)
	}
	if err != nil {
		return hiddenTagDefaultValue, err
	}
	value := strings.TrimSpace(settingRow.Value)
	if value == "" {
		return hiddenTagDefaultValue, nil
	}
	exists, err := client.HiddenTagFilter.Query().
		Where(hiddentagfilter.ValueEQ(value)).
		Exist(ctx)
	if err != nil {
		return hiddenTagDefaultValue, err
	}
	if !exists {
		if err := SetActiveHiddenTagFilterValue(ctx, client, hiddenTagDefaultValue); err != nil {
			return hiddenTagDefaultValue, err
		}
		return hiddenTagDefaultValue, nil
	}
	return value, nil
}

// ActiveHiddenTagFilter returns the ent entity representing the active hidden tag filter.
func ActiveHiddenTagFilter(ctx context.Context, client *ent.Client) (*ent.HiddenTagFilter, error) {
	value, err := ActiveHiddenTagFilterValue(ctx, client)
	if err != nil {
		return nil, err
	}
	return client.HiddenTagFilter.Query().
		Where(hiddentagfilter.ValueEQ(value)).
		Only(ctx)
}

// DefaultHiddenTagFilter returns the built-in empty filter.
func DefaultHiddenTagFilter(ctx context.Context, client *ent.Client) (*ent.HiddenTagFilter, error) {
	return client.HiddenTagFilter.Query().
		Where(hiddentagfilter.ValueEQ(hiddenTagDefaultValue)).
		Only(ctx)
}
