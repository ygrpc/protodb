package protodb

// not in db
func (x *PDBField) IsNotDB() bool {
	return x.NotDB
}

// is primary key
func (x *PDBField) IsPrimary() bool {
	return x.Primary
}

// is auto increment
func (x *PDBField) IsAutoIncrement() bool {
	return x.SerialType != 0
}

// is serial type == isautoincrement
func (x *PDBField) IsSerial() bool {
	return x.SerialType != 0
}

// is unique
func (x *PDBField) IsUnique() bool {
	return x.Unique
}

// is not null
func (x *PDBField) IsNotNull() bool {
	return x.NotNull
}

// is reference
func (x *PDBField) IsReference() bool {
	return len(x.Reference) > 0
}

// is no update
func (x *PDBField) IsNoUpdate() bool {
	return x.NoUpdate
}

// zero as null
func (x *PDBField) IsZeroAsNull() bool {
	return x.ZeroAsNull
}

// need in insert
func (x *PDBField) NeedInInsert() bool {
	if x.NotDB {
		return false
	}
	if x.NoInsert {
		return false
	}

	if x.IsAutoIncrement() {
		return false
	}

	return true
}

// need in update
func (x *PDBField) NeedInUpdate() bool {
	if x.NotDB {
		return false
	}

	if x.NoUpdate {
		return false
	}

	if x.Primary {
		return false
	}

	return true
}
