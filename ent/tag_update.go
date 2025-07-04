// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"era/booru/ent/media"
	"era/booru/ent/predicate"
	"era/booru/ent/tag"
	"errors"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
)

// TagUpdate is the builder for updating Tag entities.
type TagUpdate struct {
	config
	hooks    []Hook
	mutation *TagMutation
}

// Where appends a list predicates to the TagUpdate builder.
func (tu *TagUpdate) Where(ps ...predicate.Tag) *TagUpdate {
	tu.mutation.Where(ps...)
	return tu
}

// AddMediumIDs adds the "media" edge to the Media entity by IDs.
func (tu *TagUpdate) AddMediumIDs(ids ...string) *TagUpdate {
	tu.mutation.AddMediumIDs(ids...)
	return tu
}

// AddMedia adds the "media" edges to the Media entity.
func (tu *TagUpdate) AddMedia(m ...*Media) *TagUpdate {
	ids := make([]string, len(m))
	for i := range m {
		ids[i] = m[i].ID
	}
	return tu.AddMediumIDs(ids...)
}

// Mutation returns the TagMutation object of the builder.
func (tu *TagUpdate) Mutation() *TagMutation {
	return tu.mutation
}

// ClearMedia clears all "media" edges to the Media entity.
func (tu *TagUpdate) ClearMedia() *TagUpdate {
	tu.mutation.ClearMedia()
	return tu
}

// RemoveMediumIDs removes the "media" edge to Media entities by IDs.
func (tu *TagUpdate) RemoveMediumIDs(ids ...string) *TagUpdate {
	tu.mutation.RemoveMediumIDs(ids...)
	return tu
}

// RemoveMedia removes "media" edges to Media entities.
func (tu *TagUpdate) RemoveMedia(m ...*Media) *TagUpdate {
	ids := make([]string, len(m))
	for i := range m {
		ids[i] = m[i].ID
	}
	return tu.RemoveMediumIDs(ids...)
}

// Save executes the query and returns the number of nodes affected by the update operation.
func (tu *TagUpdate) Save(ctx context.Context) (int, error) {
	return withHooks(ctx, tu.sqlSave, tu.mutation, tu.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (tu *TagUpdate) SaveX(ctx context.Context) int {
	affected, err := tu.Save(ctx)
	if err != nil {
		panic(err)
	}
	return affected
}

// Exec executes the query.
func (tu *TagUpdate) Exec(ctx context.Context) error {
	_, err := tu.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (tu *TagUpdate) ExecX(ctx context.Context) {
	if err := tu.Exec(ctx); err != nil {
		panic(err)
	}
}

func (tu *TagUpdate) sqlSave(ctx context.Context) (n int, err error) {
	_spec := sqlgraph.NewUpdateSpec(tag.Table, tag.Columns, sqlgraph.NewFieldSpec(tag.FieldID, field.TypeInt))
	if ps := tu.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if tu.mutation.MediaCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2M,
			Inverse: true,
			Table:   tag.MediaTable,
			Columns: tag.MediaPrimaryKey,
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(media.FieldID, field.TypeString),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := tu.mutation.RemovedMediaIDs(); len(nodes) > 0 && !tu.mutation.MediaCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2M,
			Inverse: true,
			Table:   tag.MediaTable,
			Columns: tag.MediaPrimaryKey,
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(media.FieldID, field.TypeString),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := tu.mutation.MediaIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2M,
			Inverse: true,
			Table:   tag.MediaTable,
			Columns: tag.MediaPrimaryKey,
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
	if n, err = sqlgraph.UpdateNodes(ctx, tu.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{tag.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return 0, err
	}
	tu.mutation.done = true
	return n, nil
}

// TagUpdateOne is the builder for updating a single Tag entity.
type TagUpdateOne struct {
	config
	fields   []string
	hooks    []Hook
	mutation *TagMutation
}

// AddMediumIDs adds the "media" edge to the Media entity by IDs.
func (tuo *TagUpdateOne) AddMediumIDs(ids ...string) *TagUpdateOne {
	tuo.mutation.AddMediumIDs(ids...)
	return tuo
}

// AddMedia adds the "media" edges to the Media entity.
func (tuo *TagUpdateOne) AddMedia(m ...*Media) *TagUpdateOne {
	ids := make([]string, len(m))
	for i := range m {
		ids[i] = m[i].ID
	}
	return tuo.AddMediumIDs(ids...)
}

// Mutation returns the TagMutation object of the builder.
func (tuo *TagUpdateOne) Mutation() *TagMutation {
	return tuo.mutation
}

// ClearMedia clears all "media" edges to the Media entity.
func (tuo *TagUpdateOne) ClearMedia() *TagUpdateOne {
	tuo.mutation.ClearMedia()
	return tuo
}

// RemoveMediumIDs removes the "media" edge to Media entities by IDs.
func (tuo *TagUpdateOne) RemoveMediumIDs(ids ...string) *TagUpdateOne {
	tuo.mutation.RemoveMediumIDs(ids...)
	return tuo
}

// RemoveMedia removes "media" edges to Media entities.
func (tuo *TagUpdateOne) RemoveMedia(m ...*Media) *TagUpdateOne {
	ids := make([]string, len(m))
	for i := range m {
		ids[i] = m[i].ID
	}
	return tuo.RemoveMediumIDs(ids...)
}

// Where appends a list predicates to the TagUpdate builder.
func (tuo *TagUpdateOne) Where(ps ...predicate.Tag) *TagUpdateOne {
	tuo.mutation.Where(ps...)
	return tuo
}

// Select allows selecting one or more fields (columns) of the returned entity.
// The default is selecting all fields defined in the entity schema.
func (tuo *TagUpdateOne) Select(field string, fields ...string) *TagUpdateOne {
	tuo.fields = append([]string{field}, fields...)
	return tuo
}

// Save executes the query and returns the updated Tag entity.
func (tuo *TagUpdateOne) Save(ctx context.Context) (*Tag, error) {
	return withHooks(ctx, tuo.sqlSave, tuo.mutation, tuo.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (tuo *TagUpdateOne) SaveX(ctx context.Context) *Tag {
	node, err := tuo.Save(ctx)
	if err != nil {
		panic(err)
	}
	return node
}

// Exec executes the query on the entity.
func (tuo *TagUpdateOne) Exec(ctx context.Context) error {
	_, err := tuo.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (tuo *TagUpdateOne) ExecX(ctx context.Context) {
	if err := tuo.Exec(ctx); err != nil {
		panic(err)
	}
}

func (tuo *TagUpdateOne) sqlSave(ctx context.Context) (_node *Tag, err error) {
	_spec := sqlgraph.NewUpdateSpec(tag.Table, tag.Columns, sqlgraph.NewFieldSpec(tag.FieldID, field.TypeInt))
	id, ok := tuo.mutation.ID()
	if !ok {
		return nil, &ValidationError{Name: "id", err: errors.New(`ent: missing "Tag.id" for update`)}
	}
	_spec.Node.ID.Value = id
	if fields := tuo.fields; len(fields) > 0 {
		_spec.Node.Columns = make([]string, 0, len(fields))
		_spec.Node.Columns = append(_spec.Node.Columns, tag.FieldID)
		for _, f := range fields {
			if !tag.ValidColumn(f) {
				return nil, &ValidationError{Name: f, err: fmt.Errorf("ent: invalid field %q for query", f)}
			}
			if f != tag.FieldID {
				_spec.Node.Columns = append(_spec.Node.Columns, f)
			}
		}
	}
	if ps := tuo.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if tuo.mutation.MediaCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2M,
			Inverse: true,
			Table:   tag.MediaTable,
			Columns: tag.MediaPrimaryKey,
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(media.FieldID, field.TypeString),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := tuo.mutation.RemovedMediaIDs(); len(nodes) > 0 && !tuo.mutation.MediaCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2M,
			Inverse: true,
			Table:   tag.MediaTable,
			Columns: tag.MediaPrimaryKey,
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(media.FieldID, field.TypeString),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := tuo.mutation.MediaIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2M,
			Inverse: true,
			Table:   tag.MediaTable,
			Columns: tag.MediaPrimaryKey,
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
	_node = &Tag{config: tuo.config}
	_spec.Assign = _node.assignValues
	_spec.ScanValues = _node.scanValues
	if err = sqlgraph.UpdateNode(ctx, tuo.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{tag.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return nil, err
	}
	tuo.mutation.done = true
	return _node, nil
}
