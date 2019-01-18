package options_test

import (
	"io/ioutil"
	"os"

	"github.com/greenplum-db/gp-common-go-libs/dbconn"
	"github.com/greenplum-db/gp-common-go-libs/testhelper"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"

	"github.com/greenplum-db/gpbackup/utils"

	"github.com/greenplum-db/gpbackup/backup"
	"github.com/greenplum-db/gpbackup/options"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
)

var _ = Describe("options creation", func() {
	var (
		myflags *pflag.FlagSet
	)
	BeforeEach(func() {
		myflags = &pflag.FlagSet{}
		backup.SetFlagDefaults(myflags)
	})
	Describe("included tables", func() {
		Describe("Options initialization", func() {
			It("returns no included tables when none specified", func() {
				subject, err := options.NewOptions(myflags)
				Expect(err).To(Not(HaveOccurred()))

				includedTables := subject.GetIncludedTables()
				Expect(includedTables).To(BeEmpty())
			})
			It("returns the include tables when one table in flag", func() {
				err := myflags.Set(utils.INCLUDE_RELATION, "foo.bar")
				if err != nil {
					Fail("cannot set relation")
				}

				subject, err := options.NewOptions(myflags)
				Expect(err).To(Not(HaveOccurred()))

				includedTables := subject.GetIncludedTables()
				Expect(len(includedTables)).To(Equal(1))
				Expect(includedTables[0]).To(Equal("foo.bar"))
			})
			It("returns an include with special characters besides quote, dot and comma", func() {
				err := myflags.Set(utils.INCLUDE_RELATION, `foo '~#$%^&*()_-+[]{}><\|;:/?!\t\n.bar`)
				if err != nil {
					Fail("cannot set relation")
				}
				subject, err := options.NewOptions(myflags)
				Expect(err).To(Not(HaveOccurred()))

				includedTables := subject.GetIncludedTables()
				Expect(len(includedTables)).To(Equal(1))
				Expect(includedTables[0]).To(Equal(`foo '~#$%^&*()_-+[]{}><\|;:/?!\t\n.bar`))
			})
			It("returns all included tables when multiple individual flags provided", func() {
				err := myflags.Set(utils.INCLUDE_RELATION, "foo.bar")
				if err != nil {
					Fail("cannot set relation flag")
				}
				err = myflags.Set(utils.INCLUDE_RELATION, "bar.baz")
				if err != nil {
					Fail("cannot set relation flag")
				}
				subject, err := options.NewOptions(myflags)
				Expect(err).To(Not(HaveOccurred()))

				includedTables := subject.GetIncludedTables()
				Expect(len(includedTables)).To(Equal(2))
				Expect(includedTables[0]).To(Equal("foo.bar"))
				Expect(includedTables[1]).To(Equal("bar.baz"))
			})
			It("returns the text-file tables when specified", func() {
				file, err := ioutil.TempFile("/tmp", "gpbackup_test_options*.txt")
				Expect(err).To(Not(HaveOccurred()))
				defer func() {
					_ = os.Remove(file.Name())
				}()
				_, err = file.WriteString("myschema.mytable\n")
				Expect(err).To(Not(HaveOccurred()))
				_, err = file.WriteString("myschema.mytable2\n")
				Expect(err).To(Not(HaveOccurred()))
				err = file.Close()
				Expect(err).To(Not(HaveOccurred()))

				err = myflags.Set(utils.INCLUDE_RELATION_FILE, file.Name())
				if err != nil {
					Fail("cannot set relations file flag")
				}
				subject, err := options.NewOptions(myflags)
				Expect(err).To(Not(HaveOccurred()))

				includedTables := subject.GetIncludedTables()
				Expect(len(includedTables)).To(Equal(2))
				Expect(includedTables[0]).To(Equal("myschema.mytable"))
				Expect(includedTables[1]).To(Equal("myschema.mytable2"))
			})
			It("returns an error upon invalid inclusions", func() {
				err := myflags.Set(utils.INCLUDE_RELATION, "foo")
				if err != nil {
					Fail("cannot set relation")
				}
				_, err = options.NewOptions(myflags)
				Expect(err).To(HaveOccurred())
			})
		})
		Describe("character validation", func() {
			It("succeeds if characters are valid", func() {
				tableList := []string{"foo.bar", "foo.Bar", "FOO.Bar", "FO!@#.BAR"}
				err := options.ValidateCharacters(tableList)
				Expect(err).ToNot(HaveOccurred())
			})
			It("fails if schema and table are not separated by a dot", func() {
				tableList := []string{"foobar"}
				err := options.ValidateCharacters(tableList)
				Expect(err).To(HaveOccurred())
			})
			It("fails if at least one schema and table are not separated by a dot", func() {
				tableList := []string{"foobar", "foo.bar"}
				err := options.ValidateCharacters(tableList)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("foobar"))
			})
			It("fails if either table or schema is not specified", func() {
				schemaOnlyList := []string{"foo."}
				err := options.ValidateCharacters(schemaOnlyList)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("foo."))

				tableOnlyList := []string{".bar"}
				err = options.ValidateCharacters(tableOnlyList)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(".bar"))
			})
		})
		Describe("SeparateSchemaAndTable", func() {
			It("properly splits the strings", func() {
				tableList := []string{"foo.Bar", "FOO.Bar", "FO!@#.BAR"}
				expectedFqn := []options.FqnStruct{
					{SchemaName: `foo`, TableName: `Bar`},
					{SchemaName: `FOO`, TableName: `Bar`},
					{SchemaName: `FO!@#`, TableName: `BAR`},
				}
				resultFqn, err := options.SeparateSchemaAndTable(tableList)
				Expect(err).ToNot(HaveOccurred())
				Expect(resultFqn).To(Equal(expectedFqn))
			})
			It("fails to split TableName", func() {
				tableList := []string{"foo."}
				_, err := options.SeparateSchemaAndTable(tableList)
				Expect(err.Error()).To(ContainSubstring("foo."))
			})
			It("fails to split SchemaName", func() {
				tableList := []string{".bar"}
				_, err := options.SeparateSchemaAndTable(tableList)
				Expect(err.Error()).To(ContainSubstring(".bar"))
			})
			It("fails to split SchemaName or tableName (no '.')", func() {
				tableList := []string{"foobar"}
				_, err := options.SeparateSchemaAndTable(tableList)
				Expect(err.Error()).To(ContainSubstring("foobar"))
			})
			It("fails when there are more than one dots", func() {
				// todo in a future story, establish a way for users to escape dots to show us which one is *in* the name versus the dot that divides schemaname from tablename
				tableList := []string{"foobar.baz.bam"}
				_, err := options.SeparateSchemaAndTable(tableList)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("foobar.baz.bam"))
			})
		})

		Describe("QuoteTableNames", func() {
			var (
				conn   *dbconn.DBConn
				mockdb sqlmock.Sqlmock
			)
			BeforeEach(func() {
				conn, mockdb, _, _, _ = testhelper.SetupTestEnvironment()
			})

			It("returns empty result if given empty list", func() {
				tablenames := make([]string, 0)
				quotedTableNames, err := options.QuoteTableNames(conn, tablenames)
				Expect(err).To(Not(HaveOccurred()))
				Expect(tablenames).To(Equal(quotedTableNames))
			})
			It("returns a single result when given a single fqn", func() {
				tablenames := []string{"public.foo"}
				queryMock := mockdb.ExpectQuery("SELECT quote_ident")
				resultRows := sqlmock.NewRows([]string{"schemaname", "tablename"}).
					AddRow("public", "foo")

				queryMock.WillReturnRows(resultRows)

				quotedTableNames, err := options.QuoteTableNames(conn, tablenames)
				Expect(err).To(Not(HaveOccurred()))
				Expect(tablenames).To(Equal(quotedTableNames))
			})
			It("returns an array of correctly formatted fqn's", func() {
				tablenames := []string{"public.one", "public.two", "public.three"}

				queryMock := mockdb.ExpectQuery("SELECT quote_ident")
				resultRows := sqlmock.NewRows([]string{"schemaname", "tablename"}).
					AddRow("public", "one")
				queryMock.WillReturnRows(resultRows)

				queryMock = mockdb.ExpectQuery("SELECT quote_ident")
				resultRows = sqlmock.NewRows([]string{"schemaname", "tablename"}).
					AddRow("public", "two")
				queryMock.WillReturnRows(resultRows)

				queryMock = mockdb.ExpectQuery("SELECT quote_ident")
				resultRows = sqlmock.NewRows([]string{"schemaname", "tablename"}).
					AddRow("public", "three")
				queryMock.WillReturnRows(resultRows)

				quotedTableNames, err := options.QuoteTableNames(conn, tablenames)
				Expect(err).To(Not(HaveOccurred()))
				Expect(tablenames).To(Equal(quotedTableNames))
			})
			//	// todo handle embedded dots
			//	PIt("handles dots within schema or tablename", func() {
			//	})
			//	// todo handle embedded commas
			//	PIt("handles commas within schema or tablename", func() {
			//	})
			//	// todo handle embedded quotes
			//	PIt("handles quotes within schema or tablename", func() {
			//	})
			//
		})
	})
})
