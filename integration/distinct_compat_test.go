// Copyright 2021 FerretDB Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/FerretDB/FerretDB/v2/integration/setup"
	"github.com/FerretDB/FerretDB/v2/integration/shareddata"
)

// distinctCompatTestCase describes distinct compatibility test case.
type distinctCompatTestCase struct {
	field            string                   // required
	filter           bson.D                   // required
	resultType       CompatTestCaseResultType // defaults to NonEmptyResult
	failsForFerretDB string
}

func testDistinctCompat(t *testing.T, testCases map[string]distinctCompatTestCase) {
	t.Helper()

	// Use shared setup because distinct queries can't modify data.
	//
	// Use read-only user.
	// TODO https://github.com/FerretDB/FerretDB/issues/1025
	s := setup.SetupCompatWithOpts(t, &setup.SetupCompatOpts{
		Providers:                shareddata.AllProviders().Remove(shareddata.Scalars, shareddata.Decimal128s), // Remove providers with the same values with different types
		AddNonExistentCollection: true,
	})
	ctx, targetCollections, compatCollections := s.Ctx, s.TargetCollections, s.CompatCollections

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Helper()

			t.Parallel()

			filter := tc.filter
			require.NotNil(t, filter, "filter should be set")

			var nonEmptyResults bool
			for i := range targetCollections {
				targetCollection := targetCollections[i]
				compatCollection := compatCollections[i]

				t.Run(targetCollection.Name(), func(tt *testing.T) {
					tt.Helper()

					var t testing.TB = tt
					if tc.failsForFerretDB != "" {
						t = setup.FailsForFerretDB(tt, tc.failsForFerretDB)
					}

					var targetRes, compatRes bson.D
					targetErr := targetCollection.Database().RunCommand(ctx, bson.D{
						{"distinct", targetCollection.Name()},
						{"key", tc.field},
						{"query", tc.filter},
					}).Decode(&targetRes)
					compatErr := compatCollection.Database().RunCommand(ctx, bson.D{
						{"distinct", targetCollection.Name()},
						{"key", tc.field},
						{"query", tc.filter},
					}).Decode(&compatRes)

					if targetErr != nil {
						t.Logf("Target error: %v", targetErr)
						t.Logf("Compat error: %v", compatErr)

						targetErr = UnsetRaw(t, targetErr)
						compatErr = UnsetRaw(t, compatErr)
						assert.Equal(t, compatErr, targetErr)
						return
					}
					require.NoError(t, compatErr, "compat error; target returned no error")

					t.Logf("Compat (expected) result: %v", compatRes)
					t.Logf("Target (actual)   result: %v", targetRes)

					AssertEqualDocuments(t, compatRes, targetRes)

					if targetRes != nil || compatRes != nil {
						nonEmptyResults = true
					}
				})
			}

			switch tc.resultType {
			case NonEmptyResult:
				assert.True(t, nonEmptyResults, "expected non-empty results")
			case EmptyResult:
				assert.False(t, nonEmptyResults, "expected empty results")
			default:
				t.Fatalf("unknown result type %v", tc.resultType)
			}
		})
	}
}

func TestDistinctCompat(t *testing.T) {
	t.Parallel()

	testCases := map[string]distinctCompatTestCase{
		"EmptyField": {
			field:            "",
			filter:           bson.D{},
			resultType:       EmptyResult,
			failsForFerretDB: "https://github.com/FerretDB/FerretDB-DocumentDB/issues/309",
		},
		"IDAny": {
			field:  "_id",
			filter: bson.D{},
		},
		"IDString": {
			field:  "_id",
			filter: bson.D{{"_id", "string"}},
		},
		"IDNotExists": {
			field:  "_id",
			filter: bson.D{{"_id", "count-id-not-exists"}},
		},
		"VArray": {
			field:  "v",
			filter: bson.D{{"v", bson.D{{"$type", "array"}}}},
		},
		"VAny": {
			field:  "v",
			filter: bson.D{},
		},
		"NonExistentField": {
			field:  "field-not-exists",
			filter: bson.D{},
		},
		"DotNotation": {
			field:  "v.foo",
			filter: bson.D{},
		},
		"DotNotationArray": {
			field:  "v.array.0",
			filter: bson.D{},
		},
		"DotNotationArrayFirstLevel": {
			field:  "v.0.foo",
			filter: bson.D{},
		},
	}

	testDistinctCompat(t, testCases)
}
