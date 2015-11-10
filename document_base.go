package mongodm

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

//This is the base type each model needs for working with the ODM. Of course you can create your own base type but make sure
//that you implement the IDocumentBase type interface!
type DocumentBase struct {
	document   IDocumentBase   `json:"-" bson:"-"`
	collection *mgo.Collection `json:"-" bson:"-"`
	connection *Connection     `json:"-" bson:"-"`

	Id        bson.ObjectId `json:"id" bson:"_id,omitempty"`
	CreatedAt time.Time     `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt" bson:"updatedAt"`
	Deleted   bool          `json:"-" bson:"deleted"`
}

type m map[string]interface{}

func (self *DocumentBase) SetCollection(collection *mgo.Collection) {
	self.collection = collection
}

func (self *DocumentBase) SetDocument(document IDocumentBase) {
	self.document = document
}

func (self *DocumentBase) SetConnection(connection *Connection) {
	self.connection = connection
}

func (self *DocumentBase) GetId() bson.ObjectId {
	return self.Id
}

func (self *DocumentBase) SetId(id bson.ObjectId) {
	self.Id = id
}

func (self *DocumentBase) GetCreatedAt() time.Time {
	return self.CreatedAt
}

func (self *DocumentBase) SetCreatedAt(createdAt time.Time) {
	self.CreatedAt = createdAt
}

func (self *DocumentBase) SetUpdatedAt(updatedAt time.Time) {
	self.UpdatedAt = updatedAt
}

func (self *DocumentBase) GetUpdatedAt() time.Time {
	return self.UpdatedAt
}

func (self *DocumentBase) SetDeleted(deleted bool) {
	self.Deleted = deleted
}

func (self *DocumentBase) IsDeleted() bool {
	return self.Deleted
}

func (self *DocumentBase) AppendError(errorList *[]error, message string) {

	*errorList = append(*errorList, errors.New(message))
}

func (self *DocumentBase) Validate(Values ...interface{}) (bool, []error) {

	return self.DefaultValidate()
}

func (self *DocumentBase) DefaultValidate() (bool, []error) {

	documentValue := reflect.ValueOf(self.document).Elem()
	fieldType := documentValue.Type()
	validationErrors := make([]error, 0, 0)

	//iterate all struct fields
	for fieldIndex := 0; fieldIndex < documentValue.NumField(); fieldIndex++ {

		var minLen int
		var maxLen int
		var required bool
		var err error

		field := fieldType.Field(fieldIndex)
		fieldTag := field.Tag

		validation := strings.ToLower(fieldTag.Get("validation"))
		validationName := fieldTag.Get("json")

		minLenTag := fieldTag.Get("minLen")
		maxLenTag := fieldTag.Get("maxLen")
		requiredTag := fieldTag.Get("required")

		fieldName := fieldType.Field(fieldIndex).Name
		fieldValue := documentValue.Field(fieldIndex)

		if len(minLenTag) > 0 {

			minLen, err = strconv.Atoi(minLenTag)

			if err != nil {
				panic("Check your minLen tag - must be numeric")
			}
		}

		if len(maxLenTag) > 0 {

			maxLen, err = strconv.Atoi(maxLenTag)

			if err != nil {
				panic("Check your maxLen tag - must be numeric")
			}
		}

		if len(requiredTag) > 0 {

			required, err = strconv.ParseBool(requiredTag)

			if err != nil {
				panic("Check your required tag - must be boolean")
			}

		}

		splittedFieldName := strings.Split(validationName, ",")

		validationName = splittedFieldName[0]

		if validationName == "-" {
			validationName = strings.ToLower(fieldName)
		}

		isSet := false

		if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {

			isSet = true

		} else if fieldValue.Kind() == reflect.Slice || fieldValue.Kind() == reflect.Map {

			isSet = fieldValue.Len() > 0

		} else if fieldValue.Kind() == reflect.Interface {

			isSet = fieldValue.Interface() != nil

		} else {

			isSet = fieldValue.Interface() != reflect.Zero(reflect.TypeOf(fieldValue.Interface())).Interface()
		}

		//fmt.Println(validationName, isSet, fieldValue.Interface())

		if required && !isSet {

			self.AppendError(&validationErrors, L("validation.field_required", m{"fieldname": validationName}))
		}

		if stringFieldValue, ok := fieldValue.Interface().(string); ok {

			//regex to match a regex
			regex := regexp.MustCompile(`\/((?)(?:[^\r\n\[\/\\]|\\.|\[(?:[^\r\n\]\\]|\\.)*\])+)\/((?:g(?:im?|m)?|i(?:gm?|m)?|m(?:gi?|i)?)?)`)
			isRegex := regex.MatchString(validation)

			if isSet && minLen > 0 && len(stringFieldValue) < minLen {

				self.AppendError(&validationErrors, L("validation.field_minlen", m{"fieldname": validationName, "minlen": minLen}))

			} else if isSet && maxLen > 0 && len(stringFieldValue) > maxLen {

				self.AppendError(&validationErrors, L("validation.field_maxlen", m{"fieldname": validationName, "maxlen": maxLen}))
			}

			if isSet && isRegex && !validateRegexp(validation, stringFieldValue) {

				self.AppendError(&validationErrors, L("validation.field_invalid", m{"fieldname": validationName}))
			}

			if isSet && validation == "email" && !validateEmail(stringFieldValue) {

				self.AppendError(&validationErrors, L("validation.field_invalid", m{"fieldname": validationName}))
			}
		}
	}

	if len(validationErrors) > 0 {
		return false, validationErrors
	} else {
		return true, validationErrors
	}
}

func (self *DocumentBase) Update(content interface{}) (error, map[string]interface{}) {

	if contentBytes, ok := content.([]byte); ok {

		bufferMap := make(map[string]interface{})

		err := json.Unmarshal(contentBytes, &bufferMap)

		if err != nil {
			return err, nil
		}

		typeName := strings.ToLower(reflect.TypeOf(self.document).Elem().Name())

		if mapValue, ok := bufferMap[typeName]; ok {

			if typeMap, ok := mapValue.(map[string]interface{}); ok {

				delete(typeMap, "createdAt")
				delete(typeMap, "updatedAt")
				delete(typeMap, "id")
				delete(typeMap, "deleted")
			}

			bytes, err := json.Marshal(mapValue)

			if err != nil {
				return err, nil
			}

			err = json.Unmarshal(bytes, self.document)

			if err != nil {
				return err, nil
			}

		} else {

			/*
				err := json.Unmarshal(contentBytes, self.document)

				if err != nil {
					return err, nil
				}*/

			return errors.New("object not wrapped in typename"), nil
		}

		return nil, bufferMap

	} else if contentMap, ok := content.(map[string]interface{}); ok {

		delete(contentMap, "createdAt")
		delete(contentMap, "updatedAt")
		delete(contentMap, "id")
		delete(contentMap, "deleted")

		bytes, err := json.Marshal(contentMap)

		if err != nil {
			return err, nil
		}

		err = json.Unmarshal(bytes, self.document)

		if err != nil {
			return err, nil
		}

		return nil, nil
	}

	return nil, nil
}

//Calling this method will not remove the object from the database. Instead the deleted flag is set to true.
//So you can use bson.M{"deleted":false} in your query to filter those documents.
func (self *DocumentBase) Delete() error {

	if self.Id.Valid() {

		self.SetDeleted(true)

		return self.Save()
	}

	return errors.New("Invalid object id")
}

/*
func (self *DocumentBase) Fill(documentMap interface{}) {

	if workMap, ok := documentMap.(map[string]interface{}); ok {
		for key, value := range workMap {
			fmt.Println("key/value", key, value)
		}

		document := reflect.ValueOf(self.document).Elem()
		fieldType := document.Type()

		for fieldIndex := 0; fieldIndex < document.NumField(); fieldIndex++ {

			jsonTag := fieldType.Field(fieldIndex).Tag.Get("json")
			field := document.Field(fieldIndex)

			//fmt.Println(jsonTag)

			if len(jsonTag) > 0 && jsonTag != "-" {

				splitted := strings.Split(jsonTag, ",")

				if len(splitted[0]) > 0 {

					jsonTag = splitted[0]

					//check if key exists
					if val, ok := workMap[splitted[0]]; ok {

						fmt.Println(reflect.TypeOf(val).Kind())
						fmt.Println("field", field.Kind())

						if field.Kind() == reflect.Int {
							fmt.Println("cast", workMap[splitted[0]])
							val = reflect.New(reflect.TypeOf(1)).Elem().Interface()
						}

						field.Set(reflect.ValueOf(val))
					}
				}
			}

		}
	} else {
		fmt.Println("no")
	}
}
*/

/*
Populate works exactly like func (*Query) Populate. The only difference is that you call this method
on each model which embeds the DocumentBase type. This means that you can populate single elements or sub-sub-levels.

For example:
	User := connection.Model("User")

	user := &models.User{}

	err := User.Find().Exec(user)

	if err != nil {
		fmt.Println(err)
	}

	for _, user := range users {

		if user.FirstName == "Max" { //maybe NSA needs some information about Max's messages

			err := user.Populate("Messages")

			if err != nil {
				//some error occured
				continue
			}

			if messages, ok := user.Messages.([]*models.Message); ok {

				for _, message := range messages {

					fmt.Println(message.text)
				}
			} else {
				fmt.Println("something went wrong during cast. wrong type?")
			}
		}
	}


*/
func (self *DocumentBase) Populate(field ...string) error {

	if self.document == nil || self.collection == nil || self.connection == nil {
		panic("You have to initialize your document with *Model.New(document IDocumentBase) before using Populate()!")
	}

	query := &Query{
		collection: self.collection,
		connection: self.connection,
		query:      bson.M{},
		multiple:   false,
		populate:   field,
	}

	return query.runPopulation(reflect.ValueOf(self.document))
}

/*
This method saves all changes for a document. Populated relations are getting converted to object ID's / array of object ID's so you dont have to handle this by yourself.
Use this function also when the document was newly created, if it is not existent the method will call insert. During the save process createdAt and updatedAt gets also automatically persisted.

For example:

	User := connection.Model("User")

	user := &models.User{}

	User.New(user) //this sets the connection/collection for this type and is strongly necessary(!) (otherwise panic)

	user.FirstName = "Max"
	user.LastName = "Mustermann"

	err := user.Save()
*/
func (self *DocumentBase) Save() error {

	if self.document == nil || self.collection == nil || self.connection == nil {
		panic("You have to initialize your document with *Model.New(document IDocumentBase) before using Save()!")
	}

	//validate document first

	if valid, issues := self.document.Validate(); !valid {
		return &ValidationError{&QueryError{"Document could not be validated"}, issues}
	}

	/*
	 * "This behavior ensures that writes performed in the old session are necessarily observed
	 * when using the new session, as long as it was a strong or monotonic session.
	 * That said, it also means that long operations may cause other goroutines using the
	 * original session to wait." see: http://godoc.org/labix.org/v2/mgo#Session.Clone
	 */

	session := self.connection.session.Clone()
	defer session.Close()

	collection := session.DB(self.connection.Config.DatabaseName).C(self.collection.Name)

	reflectStruct := reflect.ValueOf(self.document).Elem()
	fieldType := reflectStruct.Type()
	bufferRegistry := make(map[reflect.Value]reflect.Value) //used for restoring after fields got serialized - we only save ids when not embedded

	/*
	 *	Iterate over all struct fields and determine
	 *	if there are any relations specified.
	 */
	for fieldIndex := 0; fieldIndex < reflectStruct.NumField(); fieldIndex++ {

		modelTag := fieldType.Field(fieldIndex).Tag.Get("model")       //the type which should be referenced
		relationTag := fieldType.Field(fieldIndex).Tag.Get("relation") //reference relation, e.g. one-to-one or one-to-many
		autoSaveTag := fieldType.Field(fieldIndex).Tag.Get("autosave") //flag if children of relation get automatically saved

		/*
		 *	Check if custom model and relation field tag is set,
		 *  otherwise ignore.
		 */
		if len(modelTag) > 0 {

			var fieldValue reflect.Value
			var autoSave bool
			var relation string

			field := reflectStruct.Field(fieldIndex)

			//determine relation type for default initialization
			if relationTag == REL_11 {
				relation = REL_11
			} else if relationTag == REL_1N {
				relation = REL_1N
			} else {
				relation = REL_11 //set one-to-one as default relation
			}

			//if nil and relation one-to-many -> init field with empty slice of object ids and continue loop
			if field.IsNil() {

				if relation == REL_1N {
					field.Set(reflect.ValueOf(make([]bson.ObjectId, 0, 0)))
				}

				continue
			}

			// Determine if relation should be autosaved
			if autoSaveTag == "true" {
				autoSave = true
			} else {
				autoSave = false //set autosave default to false
			}

			// Get element of field by checking if pointer or copy
			if field.Kind() == reflect.Ptr || field.Kind() == reflect.Interface {
				fieldValue = field.Elem()
			} else {
				fieldValue = field
			}

			/*
			 *	Detect if the field is a slice, struct or string
			 *  to handle the different types of relation. Other
			 *	types are not admitted.
			 */

			//one to many
			if fieldValue.Kind() == reflect.Slice {

				if relation != REL_1N {
					panic("Relation must be '1n' when using slices!")
				}

				sliceLen := fieldValue.Len()
				idBuffer := make([]bson.ObjectId, sliceLen, sliceLen)

				//iterate the slice
				for index := 0; index < sliceLen; index++ {

					sliceValue := fieldValue.Index(index)

					err, objectId := self.persistRelation(sliceValue, autoSave)

					if err != nil {
						return err
					}

					idBuffer[index] = objectId
				}

				/*
				 *	Store the original value and then replace
				 *  it with the generated id list. The value gets
				 *  restored after the model was saved
				 */

				bufferRegistry[field] = fieldValue
				field.Set(reflect.ValueOf(idBuffer))

				// one to one
			} else if (fieldValue.Kind() == reflect.Ptr && fieldValue.Elem().Kind() == reflect.Struct) || fieldValue.Kind() == reflect.String {

				if relation != REL_11 {
					panic("Relation must be '11' when using struct or id!")
				}

				var idBuffer bson.ObjectId

				err, objectId := self.persistRelation(fieldValue, autoSave)

				if err != nil {
					return err
				}

				idBuffer = objectId

				/*
				 *	Store the original value and then replace
				 *  it with the object id. The value gets
				 *  restored after the model was saved
				 */

				bufferRegistry[field] = fieldValue
				field.Set(reflect.ValueOf(idBuffer))

			} else {
				panic(fmt.Sprintf("DB: Following field kinds are supported for saving relations: slice, struct, string. You used %v", fieldValue.Kind()))
			}

		}

	}

	var err error

	now := time.Now()

	/*
	 *	Check if Object ID is already set.
	 * 	If yes -> Update object
	 * 	If no -> Create object
	 */
	if len(self.Id) == 0 {

		self.SetCreatedAt(now)
		self.SetUpdatedAt(now)

		self.SetId(bson.NewObjectId())

		err = collection.Insert(self.document)

		if err != nil {

			if mgo.IsDup(err) {
				err = &DuplicateError{&QueryError{fmt.Sprintf("Duplicate key")}}
			}
		}

	} else {

		self.SetUpdatedAt(now)
		_, errs := collection.UpsertId(self.Id, self.document)

		if errs != nil {

			if mgo.IsDup(errs) {
				errs = &DuplicateError{&QueryError{fmt.Sprintf("Duplicate key")}}
			} else {
				err = errs
			}
		}
	}

	/*
	 *	Restore fields which were changed
	 *	for saving progress (object deserialisation)
	 */
	for field, oldValue := range bufferRegistry {
		field.Set(oldValue)
	}

	return err
}

func (self *DocumentBase) persistRelation(value reflect.Value, autoSave bool) (error, bson.ObjectId) {

	//detect the type of the value which is stored within the slice
	switch typedValue := value.Interface().(type) {

	// deserialize objects to id
	case IDocumentBase:
		{
			//save children when flag is enabled
			if autoSave {
				err := typedValue.Save()

				if err != nil {
					return err, bson.ObjectId("")
				}
			}

			objectId := typedValue.GetId()

			if !objectId.Valid() {
				panic("DB: Can not persist the relation object because the child was not saved before (invalid id).")
			}

			return nil, objectId
		}

	//only save the id
	case bson.ObjectId:
		{
			if !typedValue.Valid() {
				panic("DB: Can not persist the relation object because the child was not saved before (invalid id).")
			}

			return nil, typedValue
		}

	default:
		{
			panic(fmt.Sprintf("DB: Only type 'bson.ObjectId' and 'IDocumentBase' can be stored in slices. You used %v", value.Interface()))
		}
	}
}
