<!-- prettier-ignore-start -->

{
  "title": "Hello, world!",
  "template": "layout",
  "permalink": "/hello-world",
  "type": "post",
  "draft": true,
  "date": "2021-09-11"
}
<!-- prettier-ignore-end -->

```go
// Concurrent computation of pi.
// See https://goo.gl/la6Kli.
//
// This demonstrates Go's ability to handle
// large numbers of concurrent processes.
// It is an unreasonable way to calculate pi.
package main

import (
	"fmt"
	"math"
)

func main() {
	fmt.Println(pi(5000))
}

// pi launches n goroutines to compute an
// approximation of pi.
func pi(n int) float64 {
	ch := make(chan float64)
	for k := 0; k < n; k++ {
		go term(ch, float64(k))
	}
	f := 0.0
	for k := 0; k < n; k++ {
		f += <-ch
	}
	return f
}

func term(ch chan float64, k float64) {
	ch <- 4 * math.Pow(-1, k) / (2*k + 1)
}
```

{{ image "/images/san-juan-mountains.jpg" "San Juan Mountains" }}

> Over the moon  
> Under the stars  
> Feel them arresting me  
> Unknowables  
> Fading at dawn  
> Troubles, too
>
> Dimness sustains  
> Oh the regret  
> I could be lost to you  
> Lost in thought  
> Sending a kiss  
> Back to the sky
>
> So has my world become  
> Run out of breath  
> I'm not the only one to lose a friend
>
> Where do you go  
> You're going home
>
> What do I do with the  
> Void in your shape  
> Leaving me frailty  
> A drop and I break
>
> What do I do  
> With half of myself  
> Then when the stars align  
> With some kind of peace
>
> I could be loved by you  
> Either way  
> Where did you go  
> You're going home
>
> Then when the stars align  
> With some kind of peace
>
> I know I'm loved by you  
> Either way  
> Where did you go  
> You're going home
>
> Then when the stars align  
> With some kind of peace
>
> I know I'm loved by you  
> Either way  
> Where did you go  
> You're going home
>
> You're going home
>
> <cite>Ólafur Arnalds, JFDR — Back To The Sky</cite>
