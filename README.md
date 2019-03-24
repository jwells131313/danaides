[//]: # " DO NOT ALTER OR REMOVE COPYRIGHT NOTICES OR THIS HEADER. "
[//]: # "  "
[//]: # " Copyright (c) 2019 Oracle and/or its affiliates. All rights reserved. "
[//]: # "  "
[//]: # " The contents of this file are subject to the terms of either the GNU "
[//]: # " General Public License Version 2 only (''GPL'') or the Common Development "
[//]: # " and Distribution License(''CDDL'') (collectively, the ''License'').  You "
[//]: # " may not use this file except in compliance with the License.  You can "
[//]: # " obtain a copy of the License at "
[//]: # " https://oss.oracle.com/licenses/CDDL+GPL-1.1 "
[//]: # " or LICENSE.txt.  See the License for the specific "
[//]: # " language governing permissions and limitations under the License. "
[//]: # "  "
[//]: # " When distributing the software, include this License Header Notice in each "
[//]: # " file and include the License file at LICENSE.txt. "
[//]: # "  "
[//]: # " GPL Classpath Exception: "
[//]: # " Oracle designates this particular file as subject to the ''Classpath'' "
[//]: # " exception as provided by Oracle in the GPL Version 2 section of the License "
[//]: # " file that accompanied this code. "
[//]: # "  "
[//]: # " Modifications: "
[//]: # " If applicable, add the following below the License Header, with the fields "
[//]: # " enclosed by brackets [] replaced by your own identifying information: "
[//]: # " ''Portions Copyright [year] [name of copyright owner]'' "
[//]: # "  "
[//]: # " Contributor(s): "
[//]: # " If you wish your version of this file to be governed by only the CDDL or "
[//]: # " only the GPL Version 2, indicate your decision by adding ''[Contributor] "
[//]: # " elects to include this software in this distribution under the [CDDL or GPL "
[//]: # " Version 2] license.''  If you don't indicate a single choice of license, a "
[//]: # " recipient has the option to distribute your version of this file under "
[//]: # " either the CDDL, the GPL Version 2 or to extend the choice of license to "
[//]: # " its licensees as provided above.  However, if you add GPL Version 2 code "
[//]: # " and therefore, elected the GPL Version 2 license, then the option applies "
[//]: # " only if the new code is made subject to such option by the copyright "
[//]: # " holder. "

# danaides [![wercker status](https://app.wercker.com/status/3af761a000be024abb16aaf7a7fc8a74/s/master "wercker status")](https://app.wercker.com/project/byKey/3af761a000be024abb16aaf7a7fc8a74)

Leaky Bucket Rate Limiter algorithm for streaming use cases

## Streaming Leaky Bucket

This implemantation of the leaky bucket algorithm is meant for streaming
use case.  The Limiter is the bucket and it has a desired data flow rate.
Chunks of data can be added to the bucket by number and the Limiter will
stream numbers elements back to the user or tell the user how much time
they need to wait before asking the limiter for more data.

Basic usage:

```
limiter := New(100) // 100 per second

limiter.Add(200)
limiter.Add(50)

for {
  took, delay := limiter.Take()
  if took == 0 && delay == 0 {
    // The limiter is empty, break out and get more data
    break
  }

  if took == 0 {
    time.Sleep(delay)
  } else {
    // do whatever it is you are doing with took elements
  }
}
```
