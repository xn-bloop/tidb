// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package executor

import (
	"context"

	. "github.com/pingcap/check"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/types"
)

func (s *pkgTestSuite) TestJoinExec(c *C) {
	colTypes := []*types.FieldType{
		types.NewFieldType(mysql.TypeLonglong),
		types.NewFieldType(mysql.TypeDouble),
	}
	casTest := defaultHashJoinTestCase(colTypes, 0, false)

	runTest := func() {
		opt1 := mockDataSourceParameters{
			rows: casTest.rows,
			ctx:  casTest.ctx,
			genDataFunc: func(row int, typ *types.FieldType) interface{} {
				switch typ.Tp {
				case mysql.TypeLong, mysql.TypeLonglong:
					return int64(row)
				case mysql.TypeDouble:
					return float64(row)
				default:
					panic("not implement")
				}
			},
		}
		opt2 := opt1
		opt1.schema = expression.NewSchema(casTest.columns()...)
		opt2.schema = expression.NewSchema(casTest.columns()...)
		dataSource1 := buildMockDataSource(opt1)
		dataSource2 := buildMockDataSource(opt2)
		dataSource1.prepareChunks()
		dataSource2.prepareChunks()

		exec := prepare4HashJoin(casTest, dataSource1, dataSource2)
		result := newFirstChunk(exec)
		{
			ctx := context.Background()
			chk := newFirstChunk(exec)
			err := exec.Open(ctx)
			c.Assert(err, IsNil)
			for {
				err = exec.Next(ctx, chk)
				c.Assert(err, IsNil)
				if chk.NumRows() == 0 {
					break
				}
				result.Append(chk, 0, chk.NumRows())
			}
			c.Assert(exec.rowContainer.alreadySpilled(), Equals, casTest.disk)
			err = exec.Close()
			c.Assert(err, IsNil)
		}

		c.Assert(result.NumCols(), Equals, 4)
		c.Assert(result.NumRows(), Equals, casTest.rows)
		visit := make(map[int64]bool, casTest.rows)
		for i := 0; i < casTest.rows; i++ {
			val := result.Column(0).Int64s()[i]
			c.Assert(result.Column(1).Float64s()[i], Equals, float64(val))
			c.Assert(result.Column(2).Int64s()[i], Equals, val)
			c.Assert(result.Column(3).Float64s()[i], Equals, float64(val))
			visit[val] = true
		}
		for i := 0; i < casTest.rows; i++ {
			c.Assert(visit[int64(i)], IsTrue)
		}
	}

	concurrency := []int{1, 4}
	rows := []int{3, 1024, 4096}
	disk := []bool{false, true}
	for _, concurrency := range concurrency {
		for _, rows := range rows {
			for _, disk := range disk {
				casTest.concurrency = concurrency
				casTest.rows = rows
				casTest.disk = disk
				runTest()
			}
		}
	}

}
