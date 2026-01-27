package schema

// GraphQL response structs
type GraphQLResp struct {
	Transactions GraphQLTransactions `json:"transactions"`
}

type GraphQLTransactions struct {
	PageInfo GraphQLPageInfo `json:"pageInfo"`
	Edges    []GraphQLEdge   `json:"edges"`
}

type GraphQLPageInfo struct {
	HasNextPage bool `json:"hasNextPage"`
}

type GraphQLEdge struct {
	Cursor string      `json:"cursor"`
	Node   GraphQLNode `json:"node"`
}

type GraphQLNode struct {
	ID   string       `json:"id"`
	Tags []GraphQLTag `json:"tags"`
}

type GraphQLTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

const QueryTmp = `
{
  transactions(
    owners: ["%s"]
    tags: [
      {name: "Process", values: ["%s"]},
      {name: "Nonce", values: [%s]}
    ]
  ){
    pageInfo {
      hasNextPage
    }
    edges {
      cursor
      node {
        id
        tags {
          name
          value
        }
      }
    }
  }
}`

const QueryMaxNonceTmp = `
{
  transactions(
    owners: ["%s"]
    tags: [
      {name: "Process", values: ["%s"]}
    ]
    sort: HEIGHT_DESC
    first: 1
  ){
    edges {
      node {
        tags {
          name
          value
        }
      }
    }
  }
}`
