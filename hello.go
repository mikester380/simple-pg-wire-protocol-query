package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
)

func main() {
	var version int32 = 196610
	var startupMsg []byte
	var contents []byte

	contents = append(contents, "user"...)
	contents = append(contents, 0)
	contents = append(contents, "postgres"...)
	contents = append(contents, 0)
	contents = append(contents, "database"...)
	contents = append(contents, 0)
	contents = append(contents, "test_db"...)
	contents = append(contents, 0)
	contents = append(contents, 0)

	startupMsg = binary.BigEndian.AppendUint32(startupMsg, uint32(len(contents) + 8))
	startupMsg = binary.BigEndian.AppendUint32(startupMsg, uint32(version))
	startupMsg = append(startupMsg, contents...)

	sck, postgresDialError := net.Dial("tcp", "127.0.0.1:5432")

	if (postgresDialError != nil) {
		fmt.Println("[tcp] error connecting to postgres server")
		return
	} 

	fmt.Println("[tcp] connected to postgres server")

	// close connection after main() returns
	defer sck.Close()

	_, startupMsgError := sck.Write(startupMsg)

	if startupMsgError != nil {
		fmt.Println("[tcp] error sending StartUpMessage")
		return
	}

	readMessageLoop: for {
		resType := make([]byte, 1)
		io.ReadFull(sck, resType)

		switch string(resType) {
			case "R":
				length := make([]byte, 4)
				io.ReadFull(sck, length)
				
				cnt := make([]byte, binary.BigEndian.Uint32(length) - 4)
				io.ReadFull(sck, cnt)

			// run-time parameter status
			case "S":
				length := make([]byte, 4)
				io.ReadFull(sck, length)

				param := make([]byte, binary.BigEndian.Uint32(length) - 4)
				io.ReadFull(sck, param)

				terminatorIndex := slices.Index(param, 0)
				fmt.Printf("%s: %s\n", string(param[:terminatorIndex]), string(param[terminatorIndex+1:]))
			
			// ready for query
			case "Z":
				length := make([]byte, 4)
				status := make([]byte, 1)

				io.ReadFull(sck, length)
				io.ReadFull(sck, status)

				var msg []byte

				msg = append(msg, "Q"...)
				query := "SELECT * FROM tasks;"

				// total msg length = 4 bytes for msg length + query bytes + 1 byte for null terminator
				msg = binary.BigEndian.AppendUint32(msg, uint32(4 + len(query) + 1))

				msg = append(msg, query...)
				msg = append(msg, 0)

				sck.Write(msg)
				fmt.Printf("\nRunning Query: %s\n", query)

			// row description
			case "T":
				length := make([]byte, 4)
				io.ReadFull(sck, length)

				cnt := make([]byte, binary.BigEndian.Uint32(length) - 4)
				io.ReadFull(sck, cnt)

				cntBuffer := bytes.NewBuffer(cnt)
	
				numOfFields := make([]byte, 2)
				cntBuffer.Read(numOfFields)

				fields := ""
				
				for fieldNum := 0; fieldNum < int(binary.BigEndian.Uint16(numOfFields)); fieldNum++ {
					field, _ := cntBuffer.ReadBytes(0)
					strField := " " + string(field[:len(field)-1]) + " "

					// pull the other contents out of the buffer (18 bytes total)
					others := make([]byte, 18)
					cntBuffer.Read(others)

					if fieldNum > 0 { fields = fields + "|" + strField } else { fields = fields + strField }
				}
				fields = "|" + fields + "|"

				fmt.Println(fields)
				fmt.Println(strings.Repeat("-", len(fields)))

			// row data
			case "D":
				length := make([]byte, 4)
				io.ReadFull(sck, length)

				cnt := make([]byte, binary.BigEndian.Uint32(length) - 4)
				io.ReadFull(sck, cnt)

				cntBuffer := bytes.NewBuffer(cnt)

				numOfColumns := make([]byte, 2)
				cntBuffer.Read(numOfColumns)

				row := ""
				for colNum := 0; colNum < int(binary.BigEndian.Uint16(numOfColumns)); colNum++ {
					colValLen := make([]byte, 4)
					cntBuffer.Read(colValLen)

					colVal := make([]byte, binary.BigEndian.Uint32(colValLen))
					cntBuffer.Read(colVal)

					strColVal := " " + string(colVal) + " "

					if colNum > 0 { row = row + "|" + strColVal } else { row = row + strColVal }
				}
				row = "|" + row + "|"

				fmt.Println(row)
				fmt.Println(strings.Repeat("-", len(row)))

			// command complete
			case "C":
				length := make([]byte, 4)
				io.ReadFull(sck, length)

				cnt := make([]byte, binary.BigEndian.Uint32(length) - 4)
				io.ReadFull(sck, cnt)

				fmt.Printf("\n%s command completed.", string(cnt))
				break readMessageLoop

			default:
				length := make([]byte, 4)
				io.ReadFull(sck, length)

				cnt := make([]byte, binary.BigEndian.Uint32(length) - 4)
				io.ReadFull(sck, cnt)

				// fmt.Println(string(cnt))
		}
	}
}