package corridors

import (
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/jmoiron/sqlx"

	"github.com/lncapital/torq/internal/database"
)

func GetCorridorsReferencingCategory(db *sqlx.DB, categoryId int) (corridors []*Corridor, err error) {
	err = db.Select(&corridors, `
		SELECT *
		FROM corridor
		WHERE from_category_id = $1 OR to_category_id = $1 OR (reference_id = $1 AND corridor_type_id = $2);`, categoryId, Category().CorridorTypeId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return corridors, nil
		}
		return nil, errors.Wrap(err, database.SqlExecutionError)
	}
	return corridors, nil
}

func GetCorridorsReferencingTag(db *sqlx.DB, tagId int) (corridors []*Corridor, err error) {
	err = db.Select(&corridors, `
		SELECT *
		FROM corridor
		WHERE from_tag_id = $1 OR to_tag_id = $1 OR (reference_id = $1 AND corridor_type_id = $2);`, tagId, Tag().CorridorTypeId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return corridors, nil
		}
		return nil, errors.Wrap(err, database.SqlExecutionError)
	}
	return corridors, nil
}

func GetCorridor(db *sqlx.DB, corridorId int) (Corridor, error) {
	co := Corridor{}
	err := db.Get(&co, "SELECT * FROM corridor WHERE corridor_id = $1;", corridorId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Corridor{}, nil
		}
		return Corridor{}, errors.Wrap(err, database.SqlExecutionError)
	}
	return co, nil
}

func GetCorridorsReferencingNode(db *sqlx.DB, nodeId int) (corridors []*Corridor, err error) {
	err = db.Select(&corridors, `
		SELECT *
		FROM corridor
		WHERE from_node_id = $1 OR to_node_id = $1;`, nodeId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return corridors, nil
		}
		return nil, errors.Wrap(err, database.SqlExecutionError)
	}
	return corridors, nil
}

func getCorridorsByCorridorTypeId(db *sqlx.DB, corridorTypeId int) (corridors []*Corridor, err error) {
	return getCorridorsByCorridorType(db, *getCorridorTypeFromId(corridorTypeId))
}

func getCorridorsByTagId(db *sqlx.DB, tagId int) (corridors []*CorridorFields, err error) {
	err = db.Select(&corridors, `
		SELECT corridor_id, reference_id, last(ne.alias, ne.timestamp) as alias, ch.short_channel_id
    	FROM corridor co
    	LEFT JOIN node_event ne ON co.from_node_id = ne.event_node_id
    	LEFT JOIN channel ch ON ch.channel_id = co.channel_id WHERE reference_id = $1
		GROUP BY corridor_id, reference_id, ch.short_channel_id;`, tagId)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return corridors, nil
		}
		return nil, errors.Wrap(err, database.SqlExecutionError)
	}
	return corridors, nil
}

func getCorridorsNodesByTagId(db *sqlx.DB, tagId int) (totalNodes int, err error) {
	err = db.Get(&totalNodes, `SELECT COUNT(corridor_id) FROM corridor  WHERE reference_id = $1 AND channel_id IS NULL;`, tagId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return totalNodes, nil
		}
		return 0, errors.Wrap(err, database.SqlExecutionError)
	}
	return totalNodes, nil
}

func getCorridorsChannelsByTagId(db *sqlx.DB, tagId int) (totalChannels int, err error) {
	err = db.Get(&totalChannels, `SELECT COUNT(corridor_id) FROM corridor WHERE reference_id = $1 AND channel_id IS NOT NULL;`, tagId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return totalChannels, nil
		}
		return 0, errors.Wrap(err, database.SqlExecutionError)
	}
	return totalChannels, nil
}

func getCorridorsByCorridorType(db *sqlx.DB, corridorType CorridorType) (corridors []*Corridor, err error) {
	err = db.Select(&corridors, `SELECT * FROM corridor WHERE corridor_type_id = $1;`, corridorType.CorridorTypeId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return corridors, nil
		}
		return nil, errors.Wrap(err, database.SqlExecutionError)
	}
	return corridors, nil
}

// AddCorridor doesn't refresh the cache!!!
func AddCorridor(db *sqlx.DB, c Corridor) (*Corridor, error) {
	c.CreatedOn = time.Now().UTC()
	c.UpdateOn = c.CreatedOn
	c.Priority = calculatePriority(c)

	qb := sq.Select("count(*)").
		From("corridor").
		PlaceholderFormat(sq.Dollar).
		Where(sq.Eq{"corridor_type_id": c.CorridorTypeId})
	if c.ReferenceId != nil {
		qb = qb.Where(sq.Eq{"reference_id": *c.ReferenceId})
	} else {
		qb = qb.Where(sq.Eq{"reference_id": nil})
	}
	if c.FromCategoryId != nil {
		qb = qb.Where(sq.Eq{"from_category_id": *c.FromCategoryId})
	} else {
		qb = qb.Where(sq.Eq{"from_category_id": nil})
	}
	if c.FromTagId != nil {
		qb = qb.Where(sq.Eq{"from_tag_id": *c.FromTagId})
	} else {
		qb = qb.Where(sq.Eq{"from_tag_id": nil})
	}
	if c.FromNodeId != nil {
		qb = qb.Where(sq.Eq{"from_node_id": *c.FromNodeId})
	} else {
		qb = qb.Where(sq.Eq{"from_node_id": nil})
	}
	if c.ToCategoryId != nil {
		qb = qb.Where(sq.Eq{"to_category_id": *c.ToCategoryId})
	} else {
		qb = qb.Where(sq.Eq{"to_category_id": nil})
	}
	if c.ToTagId != nil {
		qb = qb.Where(sq.Eq{"to_tag_id": *c.ToTagId})
	} else {
		qb = qb.Where(sq.Eq{"to_tag_id": nil})
	}
	if c.ToNodeId != nil {
		qb = qb.Where(sq.Eq{"to_node_id": *c.ToNodeId})
	} else {
		qb = qb.Where(sq.Eq{"to_node_id": nil})
	}
	if c.ChannelId != nil {
		qb = qb.Where(sq.Eq{"channel_id": *c.ChannelId})
	} else {
		qb = qb.Where(sq.Eq{"channel_id": nil})
	}
	qbS, args, err := qb.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, database.SqlExecutionError)
	}
	var count int
	err = db.Get(&count, qbS, args...)
	if err != nil {
		return nil, errors.Wrap(err, database.SqlExecutionError)
	}

	if count == 0 {
		err = db.QueryRowx(`INSERT INTO corridor (corridor_type_id, reference_id, flag, inverse, priority,
                      from_category_id, from_tag_id, from_node_id, to_category_id, to_tag_id, to_node_id, channel_id,
                      created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) RETURNING corridor_id;`,
			c.CorridorTypeId, c.ReferenceId, c.Flag, c.Inverse, c.Priority,
			c.FromCategoryId, c.FromTagId, c.FromNodeId, c.ToCategoryId, c.ToTagId, c.ToNodeId, c.ChannelId,
			c.CreatedOn, c.UpdateOn).
			Scan(&c.CorridorId)
		if err != nil {
			return nil, errors.Wrap(err, database.SqlExecutionError)
		}
	} else {
		return nil, errors.New("Corridor already exists.")
	}
	return &c, nil
}

// RemoveCorridor doesn't refresh the cache!!!
func RemoveCorridor(db *sqlx.DB, corridorId int) (int64, error) {
	res, err := db.Exec(`DELETE FROM corridor WHERE corridor_id = $1;`, corridorId)
	if err != nil {
		return 0, errors.Wrap(err, database.SqlExecutionError)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, database.SqlAffectedRowsCheckError)
	}
	return rowsAffected, nil
}

func RemoveCorridorByReference(db *sqlx.DB, ReferenceId int) (int64, error) {
	res, err := db.Exec(`DELETE FROM corridor WHERE reference_id = $1;`, ReferenceId)
	if err != nil {
		return 0, errors.Wrap(err, database.SqlExecutionError)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, database.SqlAffectedRowsCheckError)
	}
	return rowsAffected, nil
}
