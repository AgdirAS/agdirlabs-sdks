//! Cursor pagination: [`Page`], [`ListOptions`] and the auto-paginating `list_all` streams.

use std::collections::VecDeque;
use std::future::Future;

use futures_core::Stream;
use serde::{Deserialize, Deserializer, Serialize};

use crate::error::Result;

/// Options for `list` calls.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ListOptions {
    /// Page size, 1–100. The API default applies when unset.
    pub limit: Option<u32>,
    /// Cursor: the id of the last item of the previous page ([`Page::next_cursor`]).
    pub after: Option<String>,
}

impl ListOptions {
    /// Default options: first page, API default size.
    pub fn new() -> Self {
        Self::default()
    }

    /// Sets the page size (1–100).
    pub fn limit(mut self, limit: u32) -> Self {
        self.limit = Some(limit);
        self
    }

    /// Starts after the item with this id.
    pub fn after(mut self, cursor: impl Into<String>) -> Self {
        self.after = Some(cursor.into());
        self
    }
}

/// One page of a list.
#[derive(Debug, Clone, PartialEq, Serialize)]
pub struct Page<T> {
    /// The items on this page.
    pub data: Vec<T>,
    /// Whether more items follow.
    pub has_more: bool,
    /// Pass as [`ListOptions::after`] to fetch the next page; `None` on the last page.
    pub next_cursor: Option<String>,
}

/// Items with an id, so a page can compute its cursor.
pub(crate) trait Identified {
    fn id(&self) -> &str;
}

impl<'de, T: Deserialize<'de> + Identified> Deserialize<'de> for Page<T> {
    fn deserialize<D: Deserializer<'de>>(d: D) -> std::result::Result<Self, D::Error> {
        #[derive(Deserialize)]
        struct Wire<T> {
            data: Vec<T>,
            #[serde(default)]
            has_more: bool,
        }
        let wire = Wire::<T>::deserialize(d)?;
        let next_cursor = if wire.has_more {
            wire.data.last().map(|item| item.id().to_owned())
        } else {
            None
        };
        Ok(Page {
            data: wire.data,
            has_more: wire.has_more,
            next_cursor,
        })
    }
}

/// Walks every page, yielding items one by one. The stream ends after the first error.
pub(crate) fn paginate<T, F, Fut>(
    options: ListOptions,
    fetch: F,
) -> impl Stream<Item = Result<T>> + Send + Unpin + 'static
where
    T: Send + 'static,
    F: FnMut(ListOptions) -> Fut + Send + 'static,
    Fut: Future<Output = Result<Page<T>>> + Send,
{
    struct State<T, F> {
        fetch: F,
        buffer: VecDeque<T>,
        next: Option<ListOptions>,
    }

    let state = State {
        fetch,
        buffer: VecDeque::new(),
        next: Some(options),
    };
    // Boxed so the stream is `Unpin`: `while let Some(x) = stream.try_next().await?` just works.
    Box::pin(futures_util::stream::unfold(
        state,
        |mut state| async move {
            loop {
                if let Some(item) = state.buffer.pop_front() {
                    return Some((Ok(item), state));
                }
                let options = state.next.take()?;
                match (state.fetch)(options.clone()).await {
                    Ok(page) => {
                        state.next = page.next_cursor.map(|cursor| ListOptions {
                            after: Some(cursor),
                            ..options
                        });
                        state.buffer.extend(page.data);
                    }
                    Err(err) => return Some((Err(err), state)),
                }
            }
        },
    ))
}
