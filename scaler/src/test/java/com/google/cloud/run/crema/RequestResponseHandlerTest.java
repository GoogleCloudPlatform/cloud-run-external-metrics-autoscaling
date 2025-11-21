/*
 * Copyright 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package com.google.cloud.run.crema;

import static com.google.common.truth.Truth.assertThat;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import com.sun.net.httpserver.Headers;
import com.sun.net.httpserver.HttpExchange;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.util.Map;
import org.junit.Rule;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;
import org.mockito.Mock;
import org.mockito.junit.MockitoJUnit;
import org.mockito.junit.MockitoRule;

@RunWith(JUnit4.class)
public final class RequestResponseHandlerTest {

  @Rule public final MockitoRule mockito = MockitoJUnit.rule();

  @Mock private HttpExchange httpExchange;
  @Mock private Headers headers;

  @Test
  public void getRequestHeaderMap_withNullHttpExchange_returnsEmptyMap() throws IOException {
    ImmutableMap<String, String> actual = RequestResponseHandler.getRequestHeaderMap(null);

    assertThat(actual).isEmpty();
  }

  @Test
  public void getRequestHeaderMap_withNullHeaders_returnsEmptyMap() throws IOException {
    when(httpExchange.getRequestHeaders()).thenReturn(null);
    ImmutableMap<String, String> actual = RequestResponseHandler.getRequestHeaderMap(httpExchange);

    assertThat(actual).isEmpty();
  }

  @Test
  public void getRequestHeaderMap_returnsHeaderMap() throws IOException {
    when(httpExchange.getRequestHeaders()).thenReturn(headers);
    when(headers.entrySet())
        .thenReturn(
            ImmutableSet.of(
                Map.entry("key1", ImmutableList.of("value1")),
                Map.entry("key2", ImmutableList.of("value2", "value3"))));

    ImmutableMap<String, String> actual = RequestResponseHandler.getRequestHeaderMap(httpExchange);

    assertThat(actual).containsExactly("key1", "value1", "key2", "value2,value3");
  }

  @Test
  public void sendFailureResponse_respondsWith500() throws IOException {
    when(httpExchange.getResponseBody()).thenReturn(new ByteArrayOutputStream());

    RequestResponseHandler.sendFailureResponse(httpExchange, new Exception("test exception"));

    verify(httpExchange).sendResponseHeaders(eq(500), anyLong());
  }

  @Test
  public void sendSuccessResponse_respondsWith200() throws IOException {
    when(httpExchange.getResponseBody()).thenReturn(new ByteArrayOutputStream());

    RequestResponseHandler.sendSuccessResponse(httpExchange);

    verify(httpExchange).sendResponseHeaders(eq(200), anyLong());
  }
}
