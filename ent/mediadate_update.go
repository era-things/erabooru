// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"era/booru/ent/date"
	"era/booru/ent/media"
	"era/booru/ent/mediadate"
	"era/booru/ent/predicate"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
)

// MediaDateUpdate is the builder for updating MediaDate entities.
type MediaDateUpdate struct {
	config
	hooks    []Hook
	mutation *MediaDateMutation
}

// Where appends a list predicates to the MediaDateUpdate builder.
func (mdu *MediaDateUpdate) Where(ps ...predicate.MediaDate) *MediaDateUpdate {
	mdu.mutation.Where(ps...)
	return mdu
}

// SetMediaID sets the "media_id" field.
func (mdu *MediaDateUpdate) SetMediaID(s string) *MediaDateUpdate {
	mdu.mutation.SetMediaID(s)
	return mdu
}

// SetNillableMediaID sets the "media_id" field if the given value is not nil.
func (mdu *MediaDateUpdate) SetNillableMediaID(s *string) *MediaDateUpdate {
	if s != nil {
		mdu.SetMediaID(*s)
	}
	return mdu
}

// SetDateID sets the "date_id" field.
func (mdu *MediaDateUpdate) SetDateID(i int) *MediaDateUpdate {
	mdu.mutation.SetDateID(i)
	return mdu
}

// SetNillableDateID sets the "date_id" field if the given value is not nil.
func (mdu *MediaDateUpdate) SetNillableDateID(i *int) *MediaDateUpdate {
	if i != nil {
		mdu.SetDateID(*i)
	}
	return mdu
}

// SetValue sets the "value" field.
func (mdu *MediaDateUpdate) SetValue(t time.Time) *MediaDateUpdate {
	mdu.mutation.SetValue(t)
	return mdu
}

// SetNillableValue sets the "value" field if the given value is not nil.
func (mdu *MediaDateUpdate) SetNillableValue(t *time.Time) *MediaDateUpdate {
	if t != nil {
		mdu.SetValue(*t)
	}
	return mdu
}

// SetMedia sets the "media" edge to the Media entity.
func (mdu *MediaDateUpdate) SetMedia(m *Media) *MediaDateUpdate {
	return mdu.SetMediaID(m.ID)
}

// SetDate sets the "date" edge to the Date entity.
func (mdu *MediaDateUpdate) SetDate(d *Date) *MediaDateUpdate {
	return mdu.SetDateID(d.ID)
}

// Mutation returns the MediaDateMutation object of the builder.
func (mdu *MediaDateUpdate) Mutation() *MediaDateMutation {
	return mdu.mutation
}

// ClearMedia clears the "media" edge to the Media entity.
func (mdu *MediaDateUpdate) ClearMedia() *MediaDateUpdate {
	mdu.mutation.ClearMedia()
	return mdu
}

// ClearDate clears the "date" edge to the Date entity.
func (mdu *MediaDateUpdate) ClearDate() *MediaDateUpdate {
	mdu.mutation.ClearDate()
	return mdu
}

// Save executes the query and returns the number of nodes affected by the update operation.
func (mdu *MediaDateUpdate) Save(ctx context.Context) (int, error) {
	return withHooks(ctx, mdu.sqlSave, mdu.mutation, mdu.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (mdu *MediaDateUpdate) SaveX(ctx context.Context) int {
	affected, err := mdu.Save(ctx)
	if err != nil {
		panic(err)
	}
	return affected
}

// Exec executes the query.
func (mdu *MediaDateUpdate) Exec(ctx context.Context) error {
	_, err := mdu.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (mdu *MediaDateUpdate) ExecX(ctx context.Context) {
	if err := mdu.Exec(ctx); err != nil {
		panic(err)
	}
}

// check runs all checks and user-defined validators on the builder.
func (mdu *MediaDateUpdate) check() error {
	if mdu.mutation.MediaCleared() && len(mdu.mutation.MediaIDs()) > 0 {
		return errors.New(`ent: clearing a required unique edge "MediaDate.media"`)
	}
	if mdu.mutation.DateCleared() && len(mdu.mutation.DateIDs()) > 0 {
		return errors.New(`ent: clearing a required unique edge "MediaDate.date"`)
	}
	return nil
}

func (mdu *MediaDateUpdate) sqlSave(ctx context.Context) (n int, err error) {
	if err := mdu.check(); err != nil {
		return n, err
	}
	_spec := sqlgraph.NewUpdateSpec(mediadate.Table, mediadate.Columns, sqlgraph.NewFieldSpec(mediadate.FieldID, field.TypeInt))
	if ps := mdu.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if value, ok := mdu.mutation.Value(); ok {
		_spec.SetField(mediadate.FieldValue, field.TypeTime, value)
	}
	if mdu.mutation.MediaCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   mediadate.MediaTable,
			Columns: []string{mediadate.MediaColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(media.FieldID, field.TypeString),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := mdu.mutation.MediaIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   mediadate.MediaTable,
			Columns: []string{mediadate.MediaColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(media.FieldID, field.TypeString),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	if mdu.mutation.DateCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   mediadate.DateTable,
			Columns: []string{mediadate.DateColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(date.FieldID, field.TypeInt),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := mdu.mutation.DateIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   mediadate.DateTable,
			Columns: []string{mediadate.DateColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(date.FieldID, field.TypeInt),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	if n, err = sqlgraph.UpdateNodes(ctx, mdu.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{mediadate.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return 0, err
	}
	mdu.mutation.done = true
	return n, nil
}

// MediaDateUpdateOne is the builder for updating a single MediaDate entity.
type MediaDateUpdateOne struct {
	config
	fields   []string
	hooks    []Hook
	mutation *MediaDateMutation
}

// SetMediaID sets the "media_id" field.
func (mduo *MediaDateUpdateOne) SetMediaID(s string) *MediaDateUpdateOne {
	mduo.mutation.SetMediaID(s)
	return mduo
}

// SetNillableMediaID sets the "media_id" field if the given value is not nil.
func (mduo *MediaDateUpdateOne) SetNillableMediaID(s *string) *MediaDateUpdateOne {
	if s != nil {
		mduo.SetMediaID(*s)
	}
	return mduo
}

// SetDateID sets the "date_id" field.
func (mduo *MediaDateUpdateOne) SetDateID(i int) *MediaDateUpdateOne {
	mduo.mutation.SetDateID(i)
	return mduo
}

// SetNillableDateID sets the "date_id" field if the given value is not nil.
func (mduo *MediaDateUpdateOne) SetNillableDateID(i *int) *MediaDateUpdateOne {
	if i != nil {
		mduo.SetDateID(*i)
	}
	return mduo
}

// SetValue sets the "value" field.
func (mduo *MediaDateUpdateOne) SetValue(t time.Time) *MediaDateUpdateOne {
	mduo.mutation.SetValue(t)
	return mduo
}

// SetNillableValue sets the "value" field if the given value is not nil.
func (mduo *MediaDateUpdateOne) SetNillableValue(t *time.Time) *MediaDateUpdateOne {
	if t != nil {
		mduo.SetValue(*t)
	}
	return mduo
}

// SetMedia sets the "media" edge to the Media entity.
func (mduo *MediaDateUpdateOne) SetMedia(m *Media) *MediaDateUpdateOne {
	return mduo.SetMediaID(m.ID)
}

// SetDate sets the "date" edge to the Date entity.
func (mduo *MediaDateUpdateOne) SetDate(d *Date) *MediaDateUpdateOne {
	return mduo.SetDateID(d.ID)
}

// Mutation returns the MediaDateMutation object of the builder.
func (mduo *MediaDateUpdateOne) Mutation() *MediaDateMutation {
	return mduo.mutation
}

// ClearMedia clears the "media" edge to the Media entity.
func (mduo *MediaDateUpdateOne) ClearMedia() *MediaDateUpdateOne {
	mduo.mutation.ClearMedia()
	return mduo
}

// ClearDate clears the "date" edge to the Date entity.
func (mduo *MediaDateUpdateOne) ClearDate() *MediaDateUpdateOne {
	mduo.mutation.ClearDate()
	return mduo
}

// Where appends a list predicates to the MediaDateUpdate builder.
func (mduo *MediaDateUpdateOne) Where(ps ...predicate.MediaDate) *MediaDateUpdateOne {
	mduo.mutation.Where(ps...)
	return mduo
}

// Select allows selecting one or more fields (columns) of the returned entity.
// The default is selecting all fields defined in the entity schema.
func (mduo *MediaDateUpdateOne) Select(field string, fields ...string) *MediaDateUpdateOne {
	mduo.fields = append([]string{field}, fields...)
	return mduo
}

// Save executes the query and returns the updated MediaDate entity.
func (mduo *MediaDateUpdateOne) Save(ctx context.Context) (*MediaDate, error) {
	return withHooks(ctx, mduo.sqlSave, mduo.mutation, mduo.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (mduo *MediaDateUpdateOne) SaveX(ctx context.Context) *MediaDate {
	node, err := mduo.Save(ctx)
	if err != nil {
		panic(err)
	}
	return node
}

// Exec executes the query on the entity.
func (mduo *MediaDateUpdateOne) Exec(ctx context.Context) error {
	_, err := mduo.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (mduo *MediaDateUpdateOne) ExecX(ctx context.Context) {
	if err := mduo.Exec(ctx); err != nil {
		panic(err)
	}
}

// check runs all checks and user-defined validators on the builder.
func (mduo *MediaDateUpdateOne) check() error {
	if mduo.mutation.MediaCleared() && len(mduo.mutation.MediaIDs()) > 0 {
		return errors.New(`ent: clearing a required unique edge "MediaDate.media"`)
	}
	if mduo.mutation.DateCleared() && len(mduo.mutation.DateIDs()) > 0 {
		return errors.New(`ent: clearing a required unique edge "MediaDate.date"`)
	}
	return nil
}

func (mduo *MediaDateUpdateOne) sqlSave(ctx context.Context) (_node *MediaDate, err error) {
	if err := mduo.check(); err != nil {
		return _node, err
	}
	_spec := sqlgraph.NewUpdateSpec(mediadate.Table, mediadate.Columns, sqlgraph.NewFieldSpec(mediadate.FieldID, field.TypeInt))
	id, ok := mduo.mutation.ID()
	if !ok {
		return nil, &ValidationError{Name: "id", err: errors.New(`ent: missing "MediaDate.id" for update`)}
	}
	_spec.Node.ID.Value = id
	if fields := mduo.fields; len(fields) > 0 {
		_spec.Node.Columns = make([]string, 0, len(fields))
		_spec.Node.Columns = append(_spec.Node.Columns, mediadate.FieldID)
		for _, f := range fields {
			if !mediadate.ValidColumn(f) {
				return nil, &ValidationError{Name: f, err: fmt.Errorf("ent: invalid field %q for query", f)}
			}
			if f != mediadate.FieldID {
				_spec.Node.Columns = append(_spec.Node.Columns, f)
			}
		}
	}
	if ps := mduo.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if value, ok := mduo.mutation.Value(); ok {
		_spec.SetField(mediadate.FieldValue, field.TypeTime, value)
	}
	if mduo.mutation.MediaCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   mediadate.MediaTable,
			Columns: []string{mediadate.MediaColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(media.FieldID, field.TypeString),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := mduo.mutation.MediaIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   mediadate.MediaTable,
			Columns: []string{mediadate.MediaColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(media.FieldID, field.TypeString),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	if mduo.mutation.DateCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   mediadate.DateTable,
			Columns: []string{mediadate.DateColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(date.FieldID, field.TypeInt),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := mduo.mutation.DateIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   mediadate.DateTable,
			Columns: []string{mediadate.DateColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(date.FieldID, field.TypeInt),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	_node = &MediaDate{config: mduo.config}
	_spec.Assign = _node.assignValues
	_spec.ScanValues = _node.scanValues
	if err = sqlgraph.UpdateNode(ctx, mduo.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{mediadate.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return nil, err
	}
	mduo.mutation.done = true
	return _node, nil
}
